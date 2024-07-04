// The uprocsrv package implements uprocd that starts procs inside an
// inner container.  Uprocd itself runs in a realm-aganostic outer
// container; it is started by [container.StartPcontainer].
package uprocsrv

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

	"sigmaos/chunkclnt"
	"sigmaos/chunksrv"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernelclnt"
	"sigmaos/linuxsched"
	"sigmaos/netsigma"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/spproxysrv"
	"sigmaos/syncmap"
	"sigmaos/uprocsrv/binsrv"
	"sigmaos/uprocsrv/proto"
)

const DIRECT = true

// Lookup may try to read proc in a proc's procEntry before uprocsrv
// has set it.  To handle this case, procEntry has a condition
// varialble on which Lookup sleeps until uprocsrv sets proc.
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

// Uprocsrv holds the state for serving procs.
type UprocSrv struct {
	mu             sync.RWMutex
	ch             chan struct{}
	pe             *proc.ProcEnv
	ssrv           *sigmasrv.SigmaSrv
	kc             *kernelclnt.KernelClnt
	sc             *sigmaclnt.SigmaClnt
	scsc           *spproxysrv.SPProxySrvCmd
	binsrv         *exec.Cmd
	kernelId       string
	realm          sp.Trealm
	netproxy       bool
	spproxydPID    sp.Tpid
	schedPolicySet bool
	procs          *syncmap.SyncMap[int, *procEntry]
	ckclnt         *chunkclnt.ChunkClnt
}

func RunUprocSrv(kernelId string, netproxy bool, up string, spproxydPID sp.Tpid) error {
	pe := proc.GetProcEnv()
	ups := &UprocSrv{
		kernelId:    kernelId,
		netproxy:    netproxy,
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
	ups.pe.SetInnerContainerIP(sp.Tip(innerIP))

	db.DPrintf(db.UPROCD, "Run %v %v %s innerIP %s outerIP %s pe %v", kernelId, up, os.Environ(), pe.GetInnerContainerIP(), pe.GetOuterContainerIP(), pe)

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	ups.sc = sc
	var ep *sp.Tendpoint
	var ssrv *sigmasrv.SigmaSrv
	if up == sp.NO_PORT.String() {
		pn := filepath.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, pe.GetPID().String())
		ssrv, err = sigmasrv.NewSigmaSrvClnt(pn, sc, ups)
		ep = ssrv.GetSigmaPSrvEndpoint()
	} else {
		var port sp.Tport
		port, err = sp.ParsePort(up)
		if err != nil {
			db.DFatalf("Error parse port: %v", err)
		}
		addr := sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, port)

		// The kernel will advertise the server, so pass "" as pn.
		ssrv, err = sigmasrv.NewSigmaSrvAddrClnt("", addr, sc, ups)
		ep = sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddrRealm(pe.GetInnerContainerIP(), sp.INNER_CONTAINER_IP, port)})
	}
	if err != nil {
		db.DFatalf("Error sigmasrvclnt: %v", err)
		return err
	}
	if err := shrinkMountTable(); err != nil {
		db.DFatalf("Error shrinking mount table: %v", err)
	}
	ups.ssrv = ssrv
	p, err := perf.NewPerf(pe, perf.UPROCD)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	var bind *binsrv.BinSrvCmd
	if DIRECT {
		go func() {
			binsrv.StartBinFs(ups, ups.kernelId, ups.pe.GetPID().String(), ups.sc, ep)
		}()
	} else {
		// Start binfsd now; when uprocds gets assigned to a realm, then
		// uprocd mounts the realm's bin directory that binfs will serve
		// from.
		bd, err := binsrv.ExecBinSrv(ups.kernelId, ups.pe.GetPID().String(), ep)
		if err != nil {
			db.DPrintf(db.ERROR, "ExecBinSrv err %v\n", err)
			return err
		}
		bind = bd
	}

	ups.ckclnt = chunkclnt.NewChunkClnt(ups.sc.FsLib)

	// Lookup the ckclnt for uprocd's local chunkd now since we will
	// need it later quickly.
	if err := ups.ckclnt.LookupEntry(ups.kernelId); err != nil {
		db.DPrintf(db.UPROCD, "LookupClnt %v %v", ups.kernelId, err)
		return err
	}

	scdp := proc.NewPrivProcPid(ups.spproxydPID, "spproxyd", nil, true)
	scdp.InheritParentProcEnv(ups.pe)
	scdp.SetHow(proc.HLINUX)
	scsc, err := spproxysrv.ExecSPProxySrv(scdp, ups.pe.GetInnerContainerIP(), ups.pe.GetOuterContainerIP(), sp.NOT_SET)
	if err != nil {
		return err
	}
	ups.scsc = scsc

	if err = ssrv.RunServer(); err != nil {
		db.DPrintf(db.UPROCD_ERR, "RunServer err %v\n", err)
		return err
	}
	db.DPrintf(db.UPROCD, "RunServer done\n")
	if !DIRECT {
		bind.Shutdown()
	}
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

func (ups *UprocSrv) setSchedPolicy(upid sp.Tpid, ptype proc.Ttype) error {
	ups.mu.RLock()
	defer ups.mu.RUnlock()

	// If already set, bail out
	if ups.schedPolicySet {
		return nil
	}

	// Promote lock
	ups.mu.RUnlock()
	ups.mu.Lock()

	// If already set, demote lock & bail out
	if ups.schedPolicySet {
		ups.mu.Unlock()
		ups.mu.RLock()
		return nil
	}

	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.setSchedPolicy: %v", upid, time.Since(start))
	}(start)

	// Set sched policy to SCHED_IDLE if running BE procs
	if ptype == proc.T_BE {
		db.DPrintf(db.UPROCD, "Set SCHED_IDLE to run %v", upid)
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
	ups.schedPolicySet = true

	// Demote to reader lock
	ups.mu.Unlock()
	ups.mu.RLock()

	return nil
}

// Set up uprocd for use for a specific realm
func (ups *UprocSrv) assignToRealm(realm sp.Trealm, upid sp.Tpid, prog string, path []string, s3secret *sp.SecretProto, ep *sp.TendpointProto) error {
	ups.mu.RLock()
	defer ups.mu.RUnlock()

	// If already assigned, bail out
	if ups.realm != sp.NOREALM {
		return nil
	}

	// Promote lock
	ups.mu.RUnlock()
	ups.mu.Lock()
	// If already assigned, demote lock & bail out
	if ups.realm != sp.NOREALM {
		ups.mu.Unlock()
		ups.mu.RLock()
		return nil
	}

	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.assignToRealm: %v", upid, time.Since(start))
	}(start)

	go func() {
		if err := ups.ckclnt.Prefetch(ups.kernelId, prog, upid, realm, path, s3secret, ep); err != nil {
			db.DPrintf(db.UPROCD, "Prefetch %v %v err %v", ups.kernelId, realm, err)
		}
	}()
	start = time.Now()
	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v", realm)

	if err := mountRealmBinDir(realm); err != nil {
		db.DFatalf("Error mount realm bin dir: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.mountRealmBinDir: %v", upid, time.Since(start))

	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v done", realm)
	// Note that the uprocsrv has been assigned.
	ups.realm = realm

	// Demote to reader lock
	ups.mu.Unlock()
	ups.mu.RLock()

	return nil
}

// Run a proc inside of an inner container
func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Run uproc %v", uproc)
	db.DPrintf(db.SPAWN_LAT, "[%v] UprocSrv.Run recvd proc time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	// Assign this uprocsrv to the realm, if not already assigned.
	if err := ups.assignToRealm(uproc.GetRealm(), uproc.GetPid(), uproc.GetVersionedProgram(), uproc.GetSigmaPath(), uproc.GetSecrets()["s3"], uproc.GetNamedEndpoint()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	// Set this uprocsrv's Linux scheduling policy
	if err := ups.setSchedPolicy(uproc.GetPid(), uproc.GetType()); err != nil {
		db.DFatalf("Err set sched policy: %v", err)
	}
	uproc.FinalizeEnv(ups.pe.GetInnerContainerIP(), ups.pe.GetOuterContainerIP(), ups.pe.GetPID())

	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc Run: spawn time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	cmd, err := container.StartUProc(uproc, ups.netproxy)
	if err != nil {
		return err
	}

	pid := cmd.Pid()
	db.DPrintf(db.UPROCD, "Pid %d\n", pid)
	pe, alloc := ups.procs.Alloc(pid, newProcEntry(uproc))
	if !alloc { // it was already inserted
		pe.insertSignal(uproc)
	}

	err = cmd.Wait()
	container.CleanupUproc(uproc.GetPid())
	ups.procs.Delete(pid)
	// ups.sc.CloseFd(pe.fd)
	return err
}

// Warm uprocd to run a program for experiments with warm start.
func (ups *UprocSrv) WarmProc(ctx fs.CtxI, req proto.WarmBinRequest, res *proto.WarmBinResult) error {
	db.DPrintf(db.UPROCD, "WarmProc %v pid %v", req, os.Getpid())
	pid := sp.Tpid(req.PidStr)
	r := sp.Trealm(req.RealmStr)
	if err := ups.assignToRealm(r, pid, req.Program, req.SigmaPath, req.GetS3Secret(), req.GetNamedEndpointProto()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	st, _, err := ups.ckclnt.GetFileStat(ups.kernelId, req.Program, pid, r, req.GetS3Secret(), req.SigmaPath)
	if err != nil {
		return err
	}
	if _, err := ups.ckclnt.FetchBinary(ups.kernelId, req.Program, pid, r, req.GetS3Secret(), st.Tsize(), req.SigmaPath); err != nil {
		return err
	}
	res.OK = true
	return nil
}

// Make and mount realm bin directory for [binsrv].
func mountRealmBinDir(realm sp.Trealm) error {
	dir := chunksrv.MkPathBinRealm(realm)
	mnt := chunksrv.PathBinProc()

	db.DPrintf(db.UPROCD, "mountRealmBinDir: %q %q\n", dir, mnt)

	if err := syscall.Mount(dir, mnt, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount realm's bin dir %q to %q err %v", dir, mnt, err)
		return err
	}
	return nil
}

func (ups *UprocSrv) Fetch(pid, cid int, prog string, sz sp.Tsize) (sp.Tsize, error) {
	pe, ok := ups.procs.Lookup(pid)
	if !ok || pe.proc == nil {
		db.DFatalf("Fetch: procs.Lookup %v %v\n", pid, prog)
	}

	db.DPrintf(db.SPAWN_LAT, "[%v] Fetch start: %q ck %d path %v time since spawn %v", pe.proc.GetPid(), ups.kernelId, cid, pe.proc.GetSigmaPath(), time.Since(pe.proc.GetSpawnTime()))

	s3secret, ok := pe.proc.GetSecrets()["s3"]
	if !ok {
		return 0, fmt.Errorf("No s3 secrets in proc")
	}

	start := time.Now()
	sz, path, err := ups.ckclnt.Fetch(ups.kernelId, prog, pe.proc.GetPid(), ups.realm, s3secret, cid, sz, pe.proc.GetSigmaPath())

	db.DPrintf(db.SPAWN_LAT, "[%v] Fetch done: %q ck %d sz %d path %q fetch lat %v; time since spawn %v", pe.proc.GetPid(), ups.kernelId, cid, sz, path, time.Since(start), time.Since(pe.proc.GetSpawnTime()))
	return sz, err
}

func (ups *UprocSrv) FetchRPC(ctx fs.CtxI, req proto.FetchRequest, res *proto.FetchResponse) error {
	db.DPrintf(db.UPROCD, "Uprocd %v Fetch %v", ups.kernelId, req)
	sz, err := ups.Fetch(int(req.Pid), int(req.ChunkId), req.Prog, sp.Tsize(req.Size))
	if err != nil {
		return err
	}
	res.Size = uint64(sz)
	return nil
}

func (ups *UprocSrv) Lookup(pid int, prog string) (*sp.Stat, error) {
	pe, alloc := ups.procs.Alloc(pid, newProcEntry(nil))
	if alloc {
		db.DPrintf(db.UPROCD, "Lookup wait for pid %v proc %v\n", pid, pe)
		pe.procWait()
	}

	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup start %v paths %v; time since spawn %v", pe.proc.GetPid(), ups.kernelId, pe.proc.GetSigmaPath(), time.Since(pe.proc.GetSpawnTime()))

	paths := pe.proc.GetSigmaPath()
	s3secret, ok := pe.proc.GetSecrets()["s3"]
	if !ok {
		return nil, fmt.Errorf("No s3 secrets in proc")
	}

	s := time.Now()
	st, path, err := ups.ckclnt.GetFileStat(ups.kernelId, prog, pe.proc.GetPid(), pe.proc.GetRealm(), s3secret, paths)
	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup done %v path %q GetFileStat lat %v; time since spawn %v", pe.proc.GetPid(), ups.kernelId, path, time.Since(s), time.Since(pe.proc.GetSpawnTime()))
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (ups *UprocSrv) LookupRPC(ctx fs.CtxI, req proto.LookupRequest, res *proto.LookupResponse) error {
	db.DPrintf(db.UPROCD, "Uprocd Lookup %v", req)

	st, err := ups.Lookup(int(req.Pid), req.Prog)
	if err != nil {
		return err
	}
	res.Stat = st.StatProto()
	return nil
}
