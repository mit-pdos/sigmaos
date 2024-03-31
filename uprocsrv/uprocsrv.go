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
	ckproto "sigmaos/chunksrv/proto"
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
}

func RunUprocSrv(kernelId string, up string) error {
	pe := proc.GetProcEnv()
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{}), pe: pe, realm: sp.NOREALM}

	db.DPrintf(db.UPROCD, "Run %v %v %s innerIP %s outerIP %s", kernelId, up, os.Environ(), pe.GetInnerContainerIP(), pe.GetOuterContainerIP())

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
	// uprocd mounts the realm's bin directory that binfs will cache
	// in and serve from.
	ups.binsrv = exec.Command("binfsd", ups.kernelId, ups.pe.GetPID().String())
	ups.binsrv.Stdout = os.Stdout
	ups.binsrv.Stderr = os.Stderr

	if err := ups.binsrv.Start(); err != nil {
		db.DPrintf(db.UPROCD, "Error start %v %v", ups.binsrv, err)
		return err
	}

	time.Sleep(1 * time.Second)

	if err = ssrv.RunServer(); err != nil {
		db.DPrintf(db.ERROR, "RunServer err %v\n", err)
	}
	db.DPrintf(db.UPROCD, "RunServer done\n")
	if ups.binsrv != nil {
		ups.binsrv.Process.Kill()
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
	return container.RunUProc(uproc)
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
	pn := binsrv.BinPath(req.Program, req.SigmaPath)
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

func (ups *UprocSrv) Fetch(ctx fs.CtxI, req ckproto.FetchRequest, res *ckproto.FetchResponse) error {
	pn := path.Join(sp.SIGMAHOME, "all-realm-bin", ups.realm.String(), req.Prog)
	db.DPrintf(db.CHUNKSRV, "Uprocd fetch %v %q", req, pn)
	return chunksrv.Fetch(ups.sc, pn, req, res)
}
