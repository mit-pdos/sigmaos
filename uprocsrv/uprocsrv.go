// The uprocsrv package implements uprocd that starts procs inside an
// inner container.  Uprocd itself runs in a realm-aganostic outer
// container; it is started by [container.StartPcontainer].
package uprocsrv

import (
	"io"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"sigmaos/binsrv"
	"sigmaos/chunksrv"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernelclnt"
	"sigmaos/netsigma"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclntsrv"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	mu       sync.RWMutex
	ch       chan struct{}
	pe       *proc.ProcEnv
	ssrv     *sigmasrv.SigmaSrv
	kc       *kernelclnt.KernelClnt
	scsc     *sigmaclntsrv.SigmaClntSrvCmd
	binsrv   *exec.Cmd
	kernelId string
	realm    sp.Trealm
	sc       *sigmaclnt.SigmaClnt
	procs    map[int]*proc.Proc
}

func RunUprocSrv(kernelId string, up string) error {
	pe := proc.GetProcEnv()
	ups := &UprocSrv{
		kernelId: kernelId,
		ch:       make(chan struct{}),
		pe:       pe,
		realm:    sp.NOREALM,
		procs:    make(map[int]*proc.Proc),
	}

	db.DPrintf(db.UPROCD, "Run %v %v %s innerIP %s outerIP %s pe %v", kernelId, up, os.Environ(), pe.GetInnerContainerIP(), pe.GetOuterContainerIP(), pe)

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	ups.sc = sc

	var ssrv *sigmasrv.SigmaSrv
	if up == sp.NO_PORT.String() {
		pn := path.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, pe.GetPID().String())
		ssrv, err = sigmasrv.NewSigmaSrv(pn, ups, pe)
	} else {
		var port sp.Tport
		port, err = sp.ParsePort(up)
		if err != nil {
			db.DFatalf("Error parse port: %v", err)
		}
		addr := sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, port, pe.GetNet())

		// The kernel will advertise the server, so pass "" as pn.
		ssrv, err = sigmasrv.NewSigmaSrvAddr("", addr, pe, ups)
	}
	if err != nil {
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

	// Start binfsd now; when uprocds gets assigned to a realm, then
	// uprocd mounts the realm's bin directory that binfs will serve
	// from.
	binsrv, err := binsrv.ExecBinSrv(ups.kernelId, ups.pe.GetPID().String())
	if err != nil {
		db.DPrintf(db.ERROR, "ExecBinSrv err %v\n", err)
		return err
	}

	if err = ssrv.RunServer(); err != nil {
		db.DPrintf(db.ERROR, "RunServer err %v\n", err)
		return err
	}
	db.DPrintf(db.UPROCD, "RunServer done\n")
	binsrv.Shutdown()
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

// Set up uprocd for use for a specific realm
func (ups *UprocSrv) assignToRealm(realm sp.Trealm, upid sp.Tpid) error {
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
	defer db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.assignToRealm: %v", upid, time.Since(start))

	start = time.Now()
	innerIP, err := netsigma.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}
	ups.pe.SetInnerContainerIP(sp.Tip(innerIP))
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.setLocalIP: %v", upid, time.Since(start))

	start = time.Now()
	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v, new innerIP %v", realm, innerIP)

	if err := mountRealmBinDir(realm); err != nil {
		db.DFatalf("Error mount realm bin dir: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.mountRealmBinDir: %v", upid, time.Since(start))

	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v done", realm)
	// Note that the uprocsrv has been assigned.
	ups.realm = realm

	// Now that the uprocd's innerIP has been established, spawn sigmaclntd
	pid := sp.GenPid("sigmaclntd")
	scdp := proc.NewPrivProcPid(pid, "sigmaclntd", nil, true)
	scdp.InheritParentProcEnv(ups.pe)
	scdp.SetHow(proc.HLINUX)
	start = time.Now()
	scsc, err := sigmaclntsrv.ExecSigmaClntSrv(scdp, ups.pe.GetInnerContainerIP(), ups.pe.GetOuterContainerIP(), sp.NOT_SET)
	if err != nil {
		return err
	}
	ups.scsc = scsc
	db.DPrintf(db.SPAWN_LAT, "[%v] execSigmaClntSrv: %v", upid, time.Since(start))

	// Demote to reader lock
	ups.mu.Unlock()
	ups.mu.RLock()

	return err
}

func (ups *UprocSrv) Assign(ctx fs.CtxI, req proto.AssignRequest, res *proto.AssignResult) error {
	// no-op
	res.OK = true
	return nil
}

// Run a proc inside of an inner container
func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Run uproc %v", uproc)
	// Assign this uprocsrv to the realm, if not already assigned.
	if err := ups.assignToRealm(uproc.GetRealm(), uproc.GetPid()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	uproc.FinalizeEnv(ups.pe.GetInnerContainerIP(), ups.pe.GetInnerContainerIP(), ups.pe.GetPID())

	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc Run: %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	cmd, err := container.StartUProc(uproc)
	if err != nil {
		return err
	}
	ups.procs[cmd.Pid()] = uproc
	err = cmd.Wait()
	delete(ups.procs, cmd.Pid())
	return err
}

// Read the binary so that binfs loads it into its cache for
// experiments with a warm cache.
func readFile(pn string) error {
	f, err := os.Open(pn)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 1024)
	for {
		_, err := f.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Warm uprocd to run a program for experiments with warm start.
func (ups *UprocSrv) WarmProc(ctx fs.CtxI, req proto.WarmBinRequest, res *proto.WarmBinResult) error {
	pn := binsrv.BinPath(req.Program)
	db.DPrintf(db.UPROCD, "WarmProc %q %v", pn, req)
	if err := ups.assignToRealm(sp.Trealm(req.RealmStr), sp.NO_PID); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	if err := readFile(pn); err != nil {
		res.OK = false
		return err
	}
	res.OK = true
	return nil
}

// Make and mount realm bin directory for [binsrv].
func mountRealmBinDir(realm sp.Trealm) error {
	dir := path.Join(sp.SIGMAHOME, "all-realm-bin", realm.String())

	// fails is already exist and if it fails for another reason Mount will fail
	if err := os.Mkdir(dir, 0750); err != nil {
		db.DPrintf(db.UPROCD, "Mkdir %q err %v\n", dir, err)
	}

	mnt := path.Join(sp.SIGMAHOME, "bin", "user")

	db.DPrintf(db.UPROCD, "mountRealmBinDir: %q %q\n", dir, mnt)

	if err := syscall.Mount(dir, mnt, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount realm's bin dir %q to %q err %v", dir, mnt, err)
		return err
	}
	return nil
}

func (ups *UprocSrv) Fetch(ctx fs.CtxI, req proto.FetchRequest, res *proto.FetchResponse) error {
	db.DPrintf(db.UPROCD, "Uprocd fetch %v", req)
	pn := path.Join(sp.SIGMAHOME, "all-realm-bin", ups.realm.String(), req.Prog)

	uproc, ok := ups.procs[int(req.Pid)]
	if !ok {
		db.DFatalf("Fetch: no proc for %d\n", req.Pid)
	}
	sz, err := chunksrv.Fetch(ups.sc, pn, req.Prog, int(req.ChunkId), uproc.GetSigmaPath())
	if err != nil {
		return err
	}
	res.Size = uint64(sz)
	return nil
}

func (ups *UprocSrv) Lookup(ctx fs.CtxI, req proto.LookupRequest, res *proto.LookupResponse) error {
	db.DPrintf(db.UPROCD, "Uprocd Lookup %v", req)
	uproc, ok := ups.procs[int(req.Pid)]
	if !ok {
		db.DFatalf("Fetch: no proc for %d\n", req.Pid)
	}
	st, err := chunksrv.Lookup(ups.sc, req.Prog, ups.kernelId, uproc.GetSigmaPath())
	if err != nil {
		return err
	}
	res.Stat = st
	return nil
}
