// The uprocsrv package implements uprocd that starts procs inside an
// inner container.  Uprocd itself runs in a realm-aganostic outer
// container; it is started by [container.StartPcontainer].
package uprocsrv

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	"sigmaos/lazypagessrv"
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
	ino  uint64
}

func newProcEntry(proc *proc.Proc, ino uint64) *procEntry {
	return &procEntry{proc: proc, ino: ino}
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

	if v, err := ups.criuInst.GetCriuVersion(); err != nil {
		db.DFatalf("GetCriuVersion err %v\n", err)
	} else {
		db.DPrintf(db.UPROCD, "GetCriuVersion %v\n", v)
	}

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
	db.DPrintf(db.UPROCD, "Run uproc %v princ %v", uproc, uproc.GetPrincipal())
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

	if uproc.GetCheckpointLocation() != "" {
		if err := ups.restoreProc(uproc); err != nil {
			return err
		}
		return nil
	} else {
		db.DPrintf(db.SPAWN_LAT, "[%v] Run uproc: spawn time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
		cmd, err := container.StartUProc(uproc, ups.netproxy)
		if err != nil {
			return err
		}
		pid := cmd.Pid()
		db.DPrintf(db.UPROCD, "Pid %d\n", pid)
		pe, alloc := ups.procs.Alloc(pid, newProcEntry(uproc, cmd.Ino()))
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
// TODO: merge with fetchBinary
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

func (ups *UprocSrv) fetchBinary(uproc *proc.Proc) error {
	prog := uproc.GetVersionedProgram()
	s3 := uproc.GetSecrets()["s3"]
	r := uproc.GetRealm()
	path := uproc.GetSigmaPath()
	pid := uproc.GetPid()
	db.DPrintf(db.UPROCD, "fetchBinary: GetFileStat %v %v", uproc, path)
	st, _, err := ups.ckclnt.GetFileStat(ups.kernelId, prog, pid, r, s3, path, uproc.GetNamedEndpoint())
	if err != nil {
		return err
	}
	db.DPrintf(db.UPROCD, "fetchBinary: FetchBinary: %v %v", uproc, path)
	if _, err := ups.ckclnt.FetchBinary(ups.kernelId, prog, pid, r, s3, st.Tsize(), path); err != nil {
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
	pe, alloc := ups.procs.Alloc(pid, newProcEntry(nil, 0))
	if alloc {
		db.DPrintf(db.UPROCD, "Lookup wait for pid %v prog %q proc %v\n", pid, prog, pe)
		pe.procWait()
	}
	return ups.lookupProc(pe.proc, prog)
}

const (
	DUMPDIR    = "/home/sigmaos/dump-"
	RESTOREDIR = "/home/sigmaos/restore-"

	CKPTLAZY = "ckptlazy"
	CKPTFULL = "ckptfull"
)

func (ups *UprocSrv) Checkpoint(ctx fs.CtxI, req proto.CheckpointProcRequest, res *proto.CheckpointProcResponse) error {
	db.DPrintf(db.UPROCD, "Checkpointing uproc %v %q", req.PidStr, req.PathName)
	spid := sp.Tpid(req.PidStr)
	pid, ok := ups.pids.Lookup(spid)
	if !ok {
		db.DPrintf(db.UPROCD, "Checkpoint no pid for %v\n", spid)
		return fmt.Errorf("no proc %v\n", spid)
	}
	pe, ok := ups.procs.Lookup(pid)
	if !ok {
		db.DPrintf(db.UPROCD, "Checkpoint no proc for %v\n", pid)
		return fmt.Errorf("no proc %v\n", spid)
	}
	imgDir := DUMPDIR + spid.String()
	err := os.MkdirAll(imgDir, os.ModePerm)
	if err != nil {
		db.DPrintf(db.CKPT, "CheckpointProc: error creating %v err %v", imgDir, err)
		return err
	}
	if err := container.CheckpointProc(ups.criuInst, pid, imgDir, spid, pe.ino); err != nil {
		return err
	}
	if err := ups.writeCheckpoint(imgDir, req.PathName, CKPTFULL); err != nil {
		db.DPrintf(db.UPROCD, "writeCheckpoint full %v\n", spid, err)
		return err
	}
	if err := ups.writeCheckpoint(imgDir, req.PathName, CKPTLAZY); err != nil {
		db.DPrintf(db.UPROCD, "writeCheckpoint lazy %v err %v\n", spid, err)
		return err
	}
	return nil
}

// Copy the checkpoint img. Depending on <ckpt> name,  copy only "pagesnonlazy-<n>.img"
func (ups *UprocSrv) writeCheckpoint(chkptLocalDir string, chkptSimgaDir string, ckpt string) error {
	ups.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777)
	pn := filepath.Join(chkptSimgaDir, ckpt)
	db.DPrintf(db.UPROCD, "writeCheckpoint: create dir: %v\n", pn)
	if err := ups.ssrv.MemFs.SigmaClnt().MkDir(pn, 0777); err != nil {
		return err
	}
	files, err := os.ReadDir(chkptLocalDir)
	if err != nil {
		db.DPrintf(db.UPROCD, "writeCheckpoint: reading local dir err %\n", err)
		return err
	}

	for _, file := range files {
		if ckpt == CKPTLAZY && strings.HasPrefix(file.Name(), "pages-") {
			continue
		}
		db.DPrintf(db.UPROCD, "writeCheckpoint: copy file %s\n", file.Name())
		b, err := os.ReadFile(filepath.Join(chkptLocalDir, file.Name()))
		if err != nil {
			db.DPrintf(db.UPROCD, "Error reading file: %v\n", err)
			return err
		}
		dstFd, err := ups.ssrv.MemFs.SigmaClnt().Create(filepath.Join(pn, file.Name()), 0777, sp.OWRITE)
		if err != nil {
			db.DPrintf(db.UPROCD, "writeCheckpoint: creating file %s err %v\n", file.Name(), err)
			return err
		}
		if _, err := ups.ssrv.MemFs.SigmaClnt().Write(dstFd, b); err != nil {
			return err
		}
		ups.ssrv.MemFs.SigmaClnt().CloseFd(dstFd)
	}
	return nil
}

func (ups *UprocSrv) restoreProc(proc *proc.Proc) error {
	dst := RESTOREDIR + proc.GetPid().String()
	ckptSigmaDir := proc.GetCheckpointLocation()
	if err := ups.readCheckpoint(ckptSigmaDir, dst, CKPTLAZY); err != nil {
		return nil
	}
	pagesId := 1
	pages := filepath.Join(ckptSigmaDir, CKPTFULL, "pages-"+strconv.Itoa(pagesId)+".img")
	// pages := filepath.Join(dst, CKPTFULL, "pages-"+strconv.Itoa(pagesId)+".img")
	if err := container.RestoreProc(ups.criuInst, proc, filepath.Join(dst, CKPTLAZY), pages); err != nil {
		return err
	}
	return nil
}

func (ups *UprocSrv) readCheckpoint(ckptSigmaDir, localDir, ckpt string) error {
	db.DPrintf(db.CKPT, "readCheckpoint %v %v %v", ckptSigmaDir, localDir, ckpt)

	os.Mkdir(localDir, 0755)
	pn := filepath.Join(localDir, ckpt)
	if err := os.Mkdir(pn, 0755); err != nil {
		db.DPrintf("Mkdir %v err %v\n", pn, err)
		return err
	}

	sts, err := ups.ssrv.MemFs.SigmaClnt().GetDir(filepath.Join(ckptSigmaDir, ckpt))
	if err != nil {
		db.DPrintf("GetDir %v err %v\n", ckptSigmaDir, err)
		return err
	}
	files := sp.Names(sts)
	for _, entry := range files {
		fn := filepath.Join(ckptSigmaDir, ckpt, entry)
		dstfn := filepath.Join(pn, entry)
		db.DPrintf(db.UPROCD, "Copy file %s to %s\n", fn, dstfn)

		rdr, err := ups.ssrv.MemFs.SigmaClnt().OpenReader(fn)
		if err != nil {
			db.DPrintf("GetFile %v err %v\n", fn, err)
			return err
		}
		file, err := os.OpenFile(dstfn, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			db.DPrintf(db.UPROCD, "OpenFile %v err %v", dstfn, err)
			return err
		}
		wrt := bufio.NewWriter(file)
		if _, err := io.Copy(wrt, rdr.Reader); err != nil {
			db.DPrintf(db.UPROCD, "Error Copy: %v", err)
			return err
		}
		rdr.Close()
		file.Close()

	}
	if ckpt == CKPTLAZY {
		db.DPrintf(db.CKPT, "Expand %s\n", pn)
		if err := lazypagessrv.ExpandLazyPages(pn); err != nil {
			db.DPrintf(db.CKPT, "ExpandLazyPages %v err %v", pn, err)
			return err
		}
	}
	return nil
}

func copyFile(srcFile, dstFile string) error {
	dst, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}
	return nil
}

func writeRestore(src, dst, ckpt string) error {
	db.DPrintf(db.CKPT, "Restore copydir %v %v %v", src, dst, ckpt)

	os.Mkdir(dst, 0755)
	pn := filepath.Join(dst, ckpt)
	os.Mkdir(pn, 0755)

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(pn, entry.Name())
		if err := copyFile(sourcePath, destPath); err != nil {
			return err
		}
	}
	if ckpt == "CKPTLAZY" {
		pageId := 1 // XXX get it from inventory
		if err := os.Remove(filepath.Join(pn, "pages-"+strconv.Itoa(pageId)+".img")); err != nil {
			db.DPrintf(db.CKPT, "copyDir: Remove err %v", err)
			return err
		}
		if err := lazypagessrv.ExpandLazyPages(pn); err != nil {
			db.DPrintf(db.CKPT, "copyDir: ExpandLazyPages err %v", err)
			return err
		}
	}
	return nil
}
