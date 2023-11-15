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
	"sigmaos/netsigma"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	mu       sync.Mutex
	ch       chan struct{}
	pcfg     *proc.ProcEnv
	ssrv     *sigmasrv.SigmaSrv
	kc       *kernelclnt.KernelClnt
	kernelId string
}

func RunUprocSrv(kernelId string, up string) error {
	pcfg := proc.GetProcEnv()
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{}), pcfg: pcfg}

	ip, _ := netsigma.LocalIP()
	db.DPrintf(db.UPROCD, "Run %v %v %s IP %s", kernelId, up, os.Environ(), ip)

	var ssrv *sigmasrv.SigmaSrv
	var err error
	if up == port.NOPORT.String() {
		pn := path.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, pcfg.GetPID().String())
		ssrv, err = sigmasrv.NewSigmaSrv(pn, ups, pcfg)
	} else {
		// The kernel will advertise the server, so pass "" as pn.
		ssrv, err = sigmasrv.NewSigmaSrvPort("", up, pcfg, ups)
	}
	if err != nil {
		return err
	}
	if err := shrinkMountTable(); err != nil {
		db.DFatalf("Error shrinking mount table: %v", err)
	}
	ups.ssrv = ssrv
	p, err := perf.NewPerf(pcfg, perf.UPROCD)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()
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

func (ups *UprocSrv) Assign(ctx fs.CtxI, req proto.AssignRequest, res *proto.AssignResult) error {
	realm := sp.Trealm(req.RealmStr)
	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v", realm)
	err := container.MountRealmBinDir(realm)
	if err != nil {
		db.DFatalf("Error mount realm bin dir: %v", err)
	}
	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v done", realm)
	res.OK = true
	return nil
}

func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Get uproc %v", uproc)
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc Run: %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	return container.RunUProc(uproc)
}
