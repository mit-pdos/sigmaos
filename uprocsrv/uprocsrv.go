package uprocsrv

import (
	"os"
	"path"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernelclnt"
	"sigmaos/keys"
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
	mu              sync.RWMutex
	ch              chan struct{}
	pe              *proc.ProcEnv
	ssrv            *sigmasrv.SigmaSrv
	kc              *kernelclnt.KernelClnt
	scsc            *sigmaclntsrv.SigmaClntSrvCmd
	kernelId        string
	realm           sp.Trealm
	assigned        bool
	sigmaclntdPID   sp.Tpid
	marshaledSCKeys []string
}

func RunUprocSrv(kernelId string, up string, sigmaclntdPID sp.Tpid, marshaledSCKeys []string, masterPubKey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) error {
	pe := proc.GetProcEnv()
	ups := &UprocSrv{
		kernelId:        kernelId,
		ch:              make(chan struct{}),
		pe:              pe,
		sigmaclntdPID:   sigmaclntdPID,
		marshaledSCKeys: marshaledSCKeys,
	}

	db.DPrintf(db.UPROCD, "Run %v %v %s innerIP %s outerIP %s", kernelId, up, os.Environ(), pe.GetInnerContainerIP(), pe.GetOuterContainerIP())

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc),
		masterPubKey,
		nil,
		sp.Tsigner(pe.GetPID()),
		pubkey,
		privkey,
	)
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf(db.ERROR, "Error NewAuthSrv %v", err)
	}
	sc.SetAuthSrv(as)
	var ssrv *sigmasrv.SigmaSrv
	if up == sp.NO_PORT.String() {
		pn := path.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, pe.GetPID().String())
		ssrv, err = sigmasrv.NewSigmaSrvClnt(pn, sc, ups)
	} else {
		var port sp.Tport
		port, err = sp.ParsePort(up)
		if err != nil {
			db.DFatalf("Error parse port: %v", err)
		}
		addr := sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, port, pe.GetNet())

		// The kernel will advertise the server, so pass "" as pn.
		ssrv, err = sigmasrv.NewSigmaSrvAddrClnt("", addr, sc, ups)
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

	if err = ssrv.RunServer(); err != nil {
		db.DPrintf(db.ERROR, "RunServer err %v\n", err)
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

func (ups *UprocSrv) assignToRealm(realm sp.Trealm, upid sp.Tpid) error {
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
	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.assignToRealm: %v", upid, time.Since(start))
	}(start)

	start = time.Now()
	innerIP, err := netsigma.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}
	ups.pe.SetInnerContainerIP(sp.Tip(innerIP))
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.setLocalIP: %v", upid, time.Since(start))

	start = time.Now()
	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v, new innerIP %v", realm, innerIP)
	err = container.MountRealmBinDir(realm)
	if err != nil {
		db.DFatalf("Error mount realm bin dir: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] uprocsrv.mountRealmBinDir: %v", upid, time.Since(start))

	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v done", realm)
	// Note that the uprocsrv has been assigned.
	ups.assigned = true

	// Now that the uprocd's innerIP has been established, spawn sigmaclntd
	scdp := proc.NewPrivProcPid(ups.sigmaclntdPID, "sigmaclntd", nil, true)
	scdp.InheritParentProcEnv(ups.pe)
	scdp.SetHow(proc.HLINUX)
	start = time.Now()
	scsc, err := sigmaclntsrv.ExecSigmaClntSrv(scdp, ups.pe.GetInnerContainerIP(), ups.pe.GetOuterContainerIP(), sp.NOT_SET, ups.marshaledSCKeys)
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
