// The msched/procsrv package implements the procsrv that starts procs inside a
// sigma container.  Procsrv itself runs in a realm-agnostic docker
// container; it is started by [container.StartDockerContainer].
package srv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	chunkclnt "sigmaos/chunk/clnt"
	chunksrv "sigmaos/chunk/srv"
	db "sigmaos/debug"
	"sigmaos/fs"
	kernelclnt "sigmaos/kernel/clnt"
	"sigmaos/linuxsched"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/sched/msched/proc/proto"
	"sigmaos/sched/msched/proc/srv/binsrv"
	"sigmaos/scontainer"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	spproxysrv "sigmaos/spproxy/srv"
	"sigmaos/util/perf"
	"sigmaos/util/syncmap"
)

// Lookup may try to read proc in a proc's procEntry before procsrv
// has set it.  To handle this case, procEntry has a condition
// varialble on which Lookup sleeps until procsrv sets proc.
type procEntry struct {
	mu   sync.Mutex
	cond *sync.Cond
	proc *proc.Proc
}

func newProcEntry(proc *proc.Proc) *procEntry {
	return &procEntry{proc: proc}
}

func (pe *procEntry) insertSignal(proc *proc.Proc) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pe.proc = proc
	if pe.cond != nil { // some thread is waiting for proc info
		pe.cond.Broadcast()
	}
}

func (pe *procEntry) procWait() {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if pe.proc == nil {
		pe.cond = sync.NewCond(&pe.mu)
	}
	for pe.proc == nil {
		pe.cond.Wait()
	}
}

// Procsrv holds the state for serving procs.
type ProcSrv struct {
	mu             sync.RWMutex
	ch             chan struct{}
	pe             *proc.ProcEnv
	ssrv           *sigmasrv.SigmaSrv
	kc             *kernelclnt.KernelClnt
	sc             *sigmaclnt.SigmaClnt
	spc            *spproxysrv.SPProxySrvCmd
	binsrv         *exec.Cmd
	kernelId       string
	realm          sp.Trealm
	dialproxy      bool
	spproxydPID    sp.Tpid
	schedPolicySet bool
	procs          *syncmap.SyncMap[int, *procEntry]
	ckclnt         *chunkclnt.ChunkClnt
}

type ProcRPCSrv struct {
	ps *ProcSrv
}

func RunProcSrv(kernelId string, dialproxy bool, spproxydPID sp.Tpid) error {
	pe := proc.GetProcEnv()
	ps := &ProcSrv{
		kernelId:    kernelId,
		dialproxy:   dialproxy,
		ch:          make(chan struct{}),
		pe:          pe,
		spproxydPID: spproxydPID,
		realm:       sp.NOREALM,
		procs:       syncmap.NewSyncMap[int, *procEntry](),
	}

	// Set inner container IP as soon as uprocsrv starts up
	innerIP, err := netsigma.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}
	ps.pe.SetInnerContainerIP(sp.Tip(innerIP))

	db.DPrintf(db.PROCD, "Run kid %v innerIP %s outerIP %s pe %v", kernelId, pe.GetInnerContainerIP(), pe.GetOuterContainerIP(), pe)

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	ps.sc = sc
	var ssrv *sigmasrv.SigmaSrv
	pn := filepath.Join(sp.MSCHED, kernelId, sp.PROCDREL, pe.GetPID().String())
	ssrv, err = sigmasrv.NewSigmaSrvClnt(pn, sc, &ProcRPCSrv{ps})
	if err != nil {
		db.DFatalf("Error sigmasrvclnt: %v", err)
		return err
	}
	if err := shrinkMountTable(); err != nil {
		db.DFatalf("Error shrinking mount table: %v", err)
	}
	ps.ssrv = ssrv
	p, err := perf.NewPerf(pe, perf.PROCD)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	go func() {
		binsrv.StartBinFs(ps)
	}()

	ps.ckclnt = chunkclnt.NewChunkClnt(ps.sc.FsLib, false)

	// Lookup the ckclnt for procd's local chunkd now since we will
	// need it later quickly.
	if err := ps.ckclnt.LookupEntry(ps.kernelId); err != nil {
		db.DPrintf(db.PROCD, "LookupClnt %v %v", ps.kernelId, err)
		return err
	}

	scdp := proc.NewPrivProcPid(ps.spproxydPID, "spproxyd", nil, true)
	scdp.InheritParentProcEnv(ps.pe)
	scdp.SetHow(proc.HLINUX)
	spc, err := spproxysrv.ExecSPProxySrv(scdp, ps.pe.GetInnerContainerIP(), ps.pe.GetOuterContainerIP(), sp.NOT_SET)
	if err != nil {
		return err
	}
	ps.spc = spc

	if err = ssrv.RunServer(); err != nil {
		db.DPrintf(db.PROCD_ERR, "RunServer err %v\n", err)
		return err
	}
	db.DPrintf(db.PROCD, "RunServer done\n")
	return nil
}

func shrinkMountTable() error {
	return nil
	mounts := []string{
		"/etc/resolv.conf",
		"/etc/hostname",
		"/etc/hosts",
		"/dev/shm",
		"/dev/mqueue",
		"/dev/pts",
		//		"/dev",
	}
	for _, mnt := range mounts {
		b := append([]byte(mnt), 0)
		_, _, errno := unix.Syscall(unix.SYS_UMOUNT2, uintptr(unsafe.Pointer(&b[0])), uintptr(0), uintptr(0))
		if errno != 0 {
			db.DFatalf("Error umount2 %v: %v", mnt, errno)
			return errno
		}
	}
	lazyUmounts := []string{
		"/sys",
	}
	for _, mnt := range lazyUmounts {
		b := append([]byte(mnt), 0)
		_, _, errno := unix.Syscall(unix.SYS_UMOUNT2, uintptr(unsafe.Pointer(&b[0])), uintptr(unix.MNT_DETACH), uintptr(0))
		if errno != 0 {
			db.DFatalf("Error umount2 %v: %v", mnt, errno)
			return errno
		}
	}
	return nil
}

func (ps *ProcSrv) setSchedPolicy(upid sp.Tpid, ptype proc.Ttype) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// If already set, bail out
	if ps.schedPolicySet {
		return nil
	}

	// Promote lock
	ps.mu.RUnlock()
	ps.mu.Lock()

	// If already set, demote lock & bail out
	if ps.schedPolicySet {
		ps.mu.Unlock()
		ps.mu.RLock()
		return nil
	}

	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.setSchedPolicy: %v", upid, time.Since(start))
	}(start)

	// Set sched policy to SCHED_IDLE if running BE procs
	if ptype == proc.T_BE {
		db.DPrintf(db.PROCD, "Set SCHED_IDLE to run %v", upid)
		attr, err := linuxsched.SchedGetAttr(0)
		if err != nil {
			db.DFatalf("Error Getattr %v", err)
			return err
		}
		attr.Policy = linuxsched.SCHED_IDLE
		err = linuxsched.SchedSetAttr(0, attr)
		if err != nil {
			db.DFatalf("Error Setattr %v", err)
			return err
		}
	}
	ps.schedPolicySet = true

	// Demote to reader lock
	ps.mu.Unlock()
	ps.mu.RLock()

	return nil
}

// Set up procd for use for a specific realm
func (ps *ProcSrv) assignToRealm(realm sp.Trealm, upid sp.Tpid, prog string, path []string, s3secret *sp.SecretProto, ep *sp.TendpointProto) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// If already assigned, bail out
	if ps.realm != sp.NOREALM {
		return nil
	}

	// Promote lock
	ps.mu.RUnlock()
	ps.mu.Lock()
	// If already assigned, demote lock & bail out
	if ps.realm != sp.NOREALM {
		ps.mu.Unlock()
		ps.mu.RLock()
		return nil
	}

	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.assignToRealm: %v", upid, time.Since(start))
	}(start)

	// Prefetch file stats
	go func() {
		s := time.Now()
		if _, _, err := ps.ckclnt.GetFileStat(ps.kernelId, prog, upid, realm, s3secret, path, ep); err != nil {
			db.DPrintf(db.PROCD, "GetFileStat %v %v err %v", ps.kernelId, realm, err)
		}
		db.DPrintf(db.SPAWN_LAT, "[%v] prefetch %v lat %v", upid, prog, time.Since(s))
	}()
	start = time.Now()
	db.DPrintf(db.PROCD, "Assign Procd to realm %v", realm)

	if err := mountRealmBinDir(realm); err != nil {
		db.DFatalf("Error mount realm bin dir: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.mountRealmBinDir: %v", upid, time.Since(start))

	db.DPrintf(db.PROCD, "Assign Procd to realm %v done", realm)
	// Note that the uprocsrv has been assigned.
	ps.realm = realm

	// Demote to reader lock
	ps.mu.Unlock()
	ps.mu.RLock()

	return nil
}

func (ps *ProcRPCSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	return ps.ps.Run(ctx, req, res)
}

// Run a proc inside of an inner container
func (ps *ProcSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.PROCD, "Run uproc %v", uproc)
	// XXX for spawn lat bench
	//	db.DPrintf(db.ALWAYS, "[%v] ProcSrv.Run recvd proc time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	db.DPrintf(db.SPAWN_LAT, "[%v] ProcSrv.Run recvd proc time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	// Spawn, but don't actually run the dummy proc
	if uproc.GetProgram() == sp.DUMMY_PROG {
		db.DPrintf(db.SPAWN_LAT, "[%v] Proc Run dummy proc: spawn time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
		db.DPrintf(db.ALWAYS, "[%v] Proc Run dummy proc: spawn time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
		// Return an error, so that the waitStart/waitExit infrastructure still
		// works
		return fmt.Errorf("Dummy")
	}
	// Assign this uprocsrv to the realm, if not already assigned.
	if err := ps.assignToRealm(uproc.GetRealm(), uproc.GetPid(), uproc.GetVersionedProgram(), uproc.GetSigmaPath(), uproc.GetSecrets()["s3"], uproc.GetNamedEndpoint()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	// Set this uprocsrv's Linux scheduling policy
	if err := ps.setSchedPolicy(uproc.GetPid(), uproc.GetType()); err != nil {
		db.DFatalf("Err set sched policy: %v", err)
	}
	uproc.FinalizeEnv(ps.pe.GetInnerContainerIP(), ps.pe.GetOuterContainerIP(), ps.pe.GetPID())
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc Run: spawn time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	cmd, err := scontainer.StartSigmaContainer(uproc, ps.dialproxy)
	if err != nil {
		return err
	}
	pid := cmd.Pid()
	db.DPrintf(db.PROCD, "Pid %v -> %d", uproc.GetPid(), pid)
	pe, alloc := ps.procs.Alloc(pid, newProcEntry(uproc))
	if !alloc { // it was already inserted
		pe.insertSignal(uproc)
	}
	err = cmd.Wait()
	if err != nil {
		db.DPrintf(db.PROCD, "[%v] Proc Run cmd.Wait err %v", uproc.GetPid(), err)
	}
	scontainer.CleanupUProc(uproc.GetPid())
	ps.procs.Delete(pid)
	// ps.sc.CloseFd(pe.fd)
	return err
}

func (ps *ProcRPCSrv) WarmProcd(ctx fs.CtxI, req proto.WarmBinRequest, res *proto.WarmBinResult) error {
	return ps.ps.WarmProcd(ctx, req, res)
}

// Warm procd to run a program for experiments with warm start.
func (ps *ProcSrv) WarmProcd(ctx fs.CtxI, req proto.WarmBinRequest, res *proto.WarmBinResult) error {
	db.DPrintf(db.PROCD, "WarmProcd %v pid %v", req, os.Getpid())
	pid := sp.Tpid(req.PidStr)
	r := sp.Trealm(req.RealmStr)
	if err := ps.assignToRealm(r, pid, req.Program, req.SigmaPath, req.GetS3Secret(), req.GetNamedEndpointProto()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	st, _, err := ps.ckclnt.GetFileStat(ps.kernelId, req.Program, pid, r, req.GetS3Secret(), req.SigmaPath, req.GetNamedEndpointProto())
	if err != nil {
		return err
	}
	if _, err := ps.ckclnt.FetchBinary(ps.kernelId, req.Program, pid, r, req.GetS3Secret(), st.Tsize(), req.SigmaPath, req.GetNamedEndpointProto()); err != nil {
		return err
	}
	res.OK = true
	return nil
}

// Make and mount realm bin directory for [binsrv].
func mountRealmBinDir(realm sp.Trealm) error {
	dir := chunksrv.MkPathBinRealm(realm)
	mnt := chunksrv.PathBinProc()

	db.DPrintf(db.PROCD, "mountRealmBinDir: %q %q\n", dir, mnt)

	if err := syscall.Mount(dir, mnt, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount realm's bin dir %q to %q err %v", dir, mnt, err)
		return err
	}
	return nil
}

func (ps *ProcSrv) Fetch(pid, cid int, prog string, sz sp.Tsize) (sp.Tsize, error) {
	pe, ok := ps.procs.Lookup(pid)
	if !ok || pe.proc == nil {
		db.DFatalf("Fetch: procs.Lookup %v %v\n", pid, prog)
	}

	db.DPrintf(db.SPAWN_LAT, "[%v] Fetch start: %q ck %d path %v time since spawn %v", pe.proc.GetPid(), ps.kernelId, cid, pe.proc.GetSigmaPath(), time.Since(pe.proc.GetSpawnTime()))

	s3secret, ok := pe.proc.GetSecrets()["s3"]
	if !ok {
		return 0, fmt.Errorf("No s3 secrets in proc")
	}

	start := time.Now()
	sz, path, err := ps.ckclnt.Fetch(ps.kernelId, prog, pe.proc.GetPid(), ps.realm, s3secret, cid, sz, pe.proc.GetSigmaPath(), pe.proc.GetNamedEndpoint())

	db.DPrintf(db.SPAWN_LAT, "[%v] Fetch done: %q ck %d sz %d path %q fetch lat %v; time since spawn %v", pe.proc.GetPid(), ps.kernelId, cid, sz, path, time.Since(start), time.Since(pe.proc.GetSpawnTime()))
	return sz, err
}

func (ps *ProcSrv) lookupProc(proc *proc.Proc, prog string) (*sp.Tstat, error) {
	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup start %v paths %v; time since spawn %v", proc.GetPid(), ps.kernelId, proc.GetSigmaPath(), time.Since(proc.GetSpawnTime()))

	paths := proc.GetSigmaPath()
	s3secret, ok := proc.GetSecrets()["s3"]
	if !ok {
		return nil, fmt.Errorf("No s3 secrets in proc")
	}

	s := time.Now()
	st, path, err := ps.ckclnt.GetFileStat(ps.kernelId, prog, proc.GetPid(), proc.GetRealm(), s3secret, paths, proc.GetNamedEndpoint())
	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup done %v path %q GetFileStat lat %v; time since spawn %v", proc.GetPid(), ps.kernelId, path, time.Since(s), time.Since(proc.GetSpawnTime()))
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (ps *ProcSrv) Lookup(pid int, prog string) (*sp.Tstat, error) {
	pe, alloc := ps.procs.Alloc(pid, newProcEntry(nil))
	if alloc {
		db.DPrintf(db.PROCD, "Lookup wait for pid %v proc %v\n", pid, pe)
		pe.procWait()
	}
	return ps.lookupProc(pe.proc, prog)
}
