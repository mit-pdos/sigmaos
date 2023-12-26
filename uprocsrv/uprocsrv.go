package uprocsrv

import (
	"os"
	"path"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernelclnt"
	"sigmaos/perf"
	"sigmaos/proc"
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
	kernelId string
	realm    sp.Trealm
	assigned bool
}

func RunUprocSrv(kernelId string, up string) error {
	pe := proc.GetProcEnv()
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{}), pe: pe}

	db.DPrintf(db.UPROCD, "Run %v %v %s IP %s", kernelId, up, os.Environ(), pe.GetLocalIP())

	var ssrv *sigmasrv.SigmaSrv
	var err error
	if up == sp.NO_PORT.String() {
		pn := path.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, pe.GetPID().String())
		ssrv, err = sigmasrv.NewSigmaSrv(pn, ups, pe)
	} else {
		var port sp.Tport
		port, err = sp.ParsePort(up)
		if err != nil {
			db.DFatalf("Error parse port: %v", err)
		}
		addr := sp.NewTaddrRealm(pe.GetLocalIP(), port, pe.GetNet())

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

	scsc, err := sigmaclntsrv.ExecSigmaClntSrv()
	if err != nil {
		return err
	}
	ups.scsc = scsc

	err = ssrv.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
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

func (ups *UprocSrv) assignToRealm(realm sp.Trealm) error {
	ups.mu.RLock()
	defer ups.mu.RUnlock()

	// If already assigned, bail out
	if ups.assigned {
		return nil
	}

	// Promote lock
	ups.mu.RUnlock()
	ups.mu.Lock()
	// If already assigned, demote lock & bail out
	if ups.assigned {
		ups.mu.Unlock()
		ups.mu.RLock()
		return nil
	}

	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v", realm)
	err := container.MountRealmBinDir(realm)
	if err != nil {
		db.DFatalf("Error mount realm bin dir: %v", err)
	}

	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v done", realm)
	// Note that the uprocsrv has been assigned.
	ups.assigned = true

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

func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Run uproc %v", uproc)
	// Assign this uprocsrv to the realm, if not already assigned.
	if err := ups.assignToRealm(uproc.GetRealm()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc Run: %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	return container.RunUProc(uproc)

}
