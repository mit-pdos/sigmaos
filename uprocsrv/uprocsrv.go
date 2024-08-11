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

	criu "github.com/checkpoint-restore/go-criu/v7"

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
	pids           *syncmap.SyncMap[sp.Tpid, int]
	ckclnt         *chunkclnt.ChunkClnt
	criuInst       *criu.Criu
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
		pids:        syncmap.NewSyncMap[sp.Tpid, int](),
		criuInst:    criu.MakeCriu(),
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
	var ssrv *sigmasrv.SigmaSrv
	if up == sp.NO_PORT.String() {
		pn := filepath.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, pe.GetPID().String())
		ssrv, err = sigmasrv.NewSigmaSrvClnt(pn, sc, ups)
	} else {
		var port sp.Tport
		port, err = sp.ParsePort(up)
		if err != nil {
			db.DFatalf("Error parse port: %v", err)
		}
		addr := sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, port)

		// The kernel will advertise the server, so pass "" as pn.
		ssrv, err = sigmasrv.NewSigmaSrvAddrClnt("", addr, sc, ups)
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

	go func() {
		binsrv.StartBinFs(ups)
	}()

	ups.ckclnt = chunkclnt.NewChunkClnt(ups.sc.FsLib, false)

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

	// Prefetch file stats
	go func() {
		s := time.Now()
		if _, _, err := ups.ckclnt.GetFileStat(ups.kernelId, prog, upid, realm, s3secret, path, ep); err != nil {
			db.DPrintf(db.UPROCD, "GetFileStat %v %v err %v", ups.kernelId, realm, err)
		}
		db.DPrintf(db.SPAWN_LAT, "[%v] prefetch %v lat %v", upid, prog, time.Since(s))
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

	restore := uproc.GetProto().ProcEnvProto.CheckpointLocation != ""
	if restore {
		db.DPrintf(db.ALWAYS, "restoring proc %v", uproc.ProcEnvProto.PidStr)
		//err := os.MkdirAll("/home/sigmaos/chkptimg", 0777)
		//if err != nil {
		//db.DPrintf(db.ALWAYS, "Error creating local chkpt dir: %s", err)
		//}
		//localChkptLoc := "/home/sigmaos/chkptimg/" + uproc.ProcEnvProto.PidStr
		// ups.readCheckpointFromS3(uproc.ProcEnvProto.CheckpointLocation, localChkptLoc)
		pid := int(uproc.ProcEnvProto.OsPid)
		if err := container.RestoreRunProc(ups.criuInst, uproc.ProcEnvProto.PidStr, pid); err != nil {
			return err
		}
		db.DPrintf(db.UPROCD, "Restored pid %d\n", pid)
		return nil
	} else {
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
		ups.pids.Insert(uproc.GetPid(), pid)
		err = cmd.Wait()
		container.CleanupUproc(uproc.GetPid())
		ups.procs.Delete(pid)
		ups.pids.Delete(uproc.GetPid())
		// ups.sc.CloseFd(pe.fd)
		return err
	}
}

// Warm uprocd to run a program for experiments with warm start.
func (ups *UprocSrv) WarmProc(ctx fs.CtxI, req proto.WarmBinRequest, res *proto.WarmBinResult) error {
	db.DPrintf(db.UPROCD, "WarmProc %v pid %v", req, os.Getpid())
	pid := sp.Tpid(req.PidStr)
	r := sp.Trealm(req.RealmStr)
	if err := ups.assignToRealm(r, pid, req.Program, req.SigmaPath, req.GetS3Secret(), req.GetNamedEndpointProto()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	st, _, err := ups.ckclnt.GetFileStat(ups.kernelId, req.Program, pid, r, req.GetS3Secret(), req.SigmaPath, nil)
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

func (ups *UprocSrv) lookupProc(proc *proc.Proc, prog string) (*sp.Stat, error) {
	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup start %v paths %v; time since spawn %v", proc.GetPid(), ups.kernelId, proc.GetSigmaPath(), time.Since(proc.GetSpawnTime()))

	paths := proc.GetSigmaPath()
	s3secret, ok := proc.GetSecrets()["s3"]
	if !ok {
		return nil, fmt.Errorf("No s3 secrets in proc")
	}

	s := time.Now()
	st, path, err := ups.ckclnt.GetFileStat(ups.kernelId, prog, proc.GetPid(), proc.GetRealm(), s3secret, paths, proc.GetNamedEndpoint())
	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup done %v path %q GetFileStat lat %v; time since spawn %v", proc.GetPid(), ups.kernelId, path, time.Since(s), time.Since(proc.GetSpawnTime()))
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (ups *UprocSrv) Lookup(pid int, prog string) (*sp.Stat, error) {
	pe, alloc := ups.procs.Alloc(pid, newProcEntry(nil))
	if alloc {
		db.DPrintf(db.UPROCD, "Lookup wait for pid %v proc %v\n", pid, pe)
		pe.procWait()
	}
	return ups.lookupProc(pe.proc, prog)
}

func (ups *UprocSrv) writeCheckpoint(chkptLocalDir string, chkptSimgaDir string) error {
	if err := ups.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777); err != nil {
		return err
	}

	db.DPrintf(db.UPROCD, "writeCheckpoint: create dir: %v\n", chkptSimgaDir)

	// loop through files in curr dir, put into files in sigmaOS dir
	files, err := os.ReadDir(chkptLocalDir)
	if err != nil {
		db.DPrintf(db.UPROCD, "writeCheckpoint: reading local dir err %\n", err)
		return err
	}

	// TODO make this just use CopyFile?
	for _, file := range files {
		db.DPrintf(db.UPROCD, "Trying to copy file %s\n", file.Name())
		dstFd, err := ups.ssrv.MemFs.SigmaClnt().Create(filepath.Join(chkptSimgaDir, file.Name()), 0777, sp.OWRITE)
		if err != nil {
			db.DPrintf(db.UPROCD, "writeCheckpoint: creating file %s err %v\n", file.Name(), err)
			return err
		}

		// write content
		fileContents, err := os.ReadFile(filepath.Join(chkptLocalDir, file.Name()))
		if err != nil {
			db.DPrintf(db.UPROCD, "writeCheckpoint: reading file err %v\n", err)
			return err
		}
		_, err = ups.ssrv.MemFs.SigmaClnt().Write(dstFd, fileContents)
		if err != nil {
			db.DPrintf(db.UPROCD, "writeCheckpoint: writing err %v\n", err)
			return err
		}

		// close
		err = ups.ssrv.MemFs.SigmaClnt().CloseFd(dstFd)
		if err != nil {
			db.DPrintf(db.UPROCD, "writeCheckpoint: close file err %v\n", err)
			return err
		}
	}
	return nil
}

func (ups *UprocSrv) Checkpoint(ctx fs.CtxI, req proto.CheckpointPidRequest, res *proto.CheckpointPidResult) error {
	db.DPrintf(db.UPROCD, "Checkpointing uproc %v %q", req.PidStr, req.PathName)

	spid := sp.Tpid(req.PidStr)
	pid, ok := ups.pids.Lookup(spid)
	if !ok {
		db.DPrintf(db.UPROCD, "Checkpoint no proc for %v\n", spid)
		return fmt.Errorf("no proc %v\n", spid)
	}

	ckptdir, err := container.CheckpointProc(ups.criuInst, pid, spid)
	if err != nil {
		return err
	}
	res.OsPid = int32(pid)
	if err = ups.writeCheckpoint(ckptdir, req.PathName); err != nil {
		// TODO clean up what was written partially?
		return err
	}
	// close chan?
	return nil
}
