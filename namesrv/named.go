package namesrv

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/keys"
	"sigmaos/leaderetcd"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

// Named implements fs/fs.go using fsetcd.  It assumes that its caller
// (protsrv) holds read/write locks.

type Named struct {
	*sigmaclnt.SigmaClnt
	*sigmasrv.SigmaSrv
	mu              sync.Mutex
	fs              *fsetcd.FsEtcd
	elect           *leaderetcd.Election
	job             string
	realm           sp.Trealm
	crash           int
	sess            *fsetcd.Session
	masterPublicKey auth.PublicKey
	masterPrivKey   auth.PrivateKey
	signer          sp.Tsigner
	pubkey          auth.PublicKey
	privkey         auth.PrivateKey
}

func toGiB(nbyte uint64) float64 {
	return float64(nbyte) / float64(1<<30)
}

func Run(args []string) error {
	//	go func() {
	//		for {
	//			time.Sleep(1000 * time.Millisecond)
	//			var ms runtime.MemStats
	//			runtime.ReadMemStats(&ms)
	//			db.DPrintf(db.ALWAYS, "Num goroutines (%v) HeapLiveBytes:(%.3f) TotalHeapAllocCum:(%3f) MaxHeapSizeEver:(%.3f) HeapNotReleasedToSys:(%.3f) HeapReleasedToSys:(%.3f) StackInuse:(%.3f) StackReqeuestedFromSys:(%.3f) SysAllocated:(%.3f)", runtime.NumGoroutine(), toGiB(ms.HeapAlloc), toGiB(ms.TotalAlloc), toGiB(ms.HeapSys), toGiB(ms.HeapIdle), toGiB(ms.HeapReleased), toGiB(ms.StackInuse), toGiB(ms.StackSys), toGiB(ms.Sys))
	//		}
	//	}()

	pe := proc.GetProcEnv()
	db.DPrintf(db.NAMED, "named started: %v cfg: %v", args, pe)
	if len(args) != 6 {
		return fmt.Errorf("%v: wrong number of arguments %v", args[0], args)
	}
	masterPubKey, pubkey, privkey, err := keys.BootstrappedKeysFromArgs(args[1:])
	if err != nil {
		db.DFatalf("Error get bootstrapped keys: %v", err)
	}
	//	masterPrivKey, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(args[2]))
	//	if err != nil {
	//		db.DFatalf("Error NewPublicKey: %v", err)
	//	}

	nd := &Named{}
	nd.masterPublicKey = masterPubKey
	nd.masterPrivKey = nil // masterPrivKey
	nd.signer = sp.Tsigner(pe.GetPID())
	nd.pubkey = pubkey
	nd.privkey = privkey
	nd.realm = sp.Trealm(args[4])
	crashing, err := strconv.Atoi(args[5])
	if err != nil {
		return fmt.Errorf("%v: crash %v isn't int", args[0], args[5])
	}
	nd.crash = crashing

	p, err := perf.NewPerf(pe, perf.NAMED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return err
	}
	nd.SigmaClnt = sc

	pn := path.Join(sp.REALMS, nd.realm.String()) + ".sem"
	sem := semclnt.NewSemClnt(nd.FsLib, pn)
	if nd.realm != sp.ROOTREALM {
		// create semaphore to signal realmd when we are the leader
		// and ready to serve requests.  realmd downs this semaphore.
		li, err := sc.LeaseClnt.AskLease(pn, fsetcd.LeaseTTL)
		if err != nil {
			return err
		}
		li.KeepExtending()
		if err := sem.InitLease(0777, li.Lease()); err != nil {
			return err
		}
	}

	nd.Started()
	ch := make(chan struct{})
	go nd.waitExit(ch)

	db.DPrintf(db.NAMED, "started %v %v", pe.GetPID(), nd.realm)

	if err := nd.startLeader(); err != nil {
		db.DPrintf(db.NAMED, "%v: startLeader %v err %v\n", pe.GetPID(), nd.realm, err)
		return err
	}
	defer nd.fs.Close()

	ep, err := nd.newSrv()
	if err != nil {
		db.DFatalf("Error newSrv %v\n", err)
	}

	db.DPrintf(db.NAMED, "newSrv %v ep %v", nd.realm, ep)

	pn = sp.NAMED
	if nd.realm == sp.ROOTREALM {
		db.DPrintf(db.ALWAYS, "SetRootNamed %v ep %v\n", nd.realm, ep)
		if err := nd.fs.SetRootNamed(ep); err != nil {
			db.DFatalf("SetNamed: %v", err)
		}
	} else {
		// note: the named proc runs in rootrealm; maybe change it XXX
		pn = path.Join(sp.REALMS, nd.realm.String())
		db.DPrintf(db.ALWAYS, "NewEndpointSymlink %v %v lid %v\n", nd.realm, pn, nd.sess.Lease())
		nd.GetAuthSrv().MintAndSetEndpointToken(ep)
		if err := nd.MkEndpointFile(pn, ep, nd.sess.Lease()); err != nil {
			db.DPrintf(db.NAMED, "MkEndpointFile %v at %v err %v\n", nd.realm, pn, err)
			return err
		}

		// Signal realmd we are ready
		if err := sem.Up(); err != nil {
			db.DPrintf(db.NAMED, "%v sem up %v err %v\n", nd.realm, sem.String(), err)
			return err
		}
	}

	nd.getRoot(pn + "/")

	if err := nd.CreateLeaderFile(path.Join(sp.NAME, nd.elect.Key()), nil, sp.TleaseId(nd.sess.Lease()), nd.elect.Fence()); err != nil {
		db.DPrintf(db.NAMED, "CreateElectionInfo %v err %v\n", nd.elect.Key(), err)
	}

	db.DPrintf(db.NAMED, "Created Leader file %v ", nd.elect.Key())

	if nd.crash > 0 {
		crash.Crasher(nd.SigmaClnt.FsLib)
	}

	<-ch

	db.DPrintf(db.ALWAYS, "%v: named done %v %v\n", pe.GetPID(), nd.realm, ep)

	if err := nd.resign(); err != nil {
		db.DPrintf(db.NAMED, "resign %v err %v\n", pe.GetPID(), err)
	}

	nd.SigmaSrv.SrvExit(proc.NewStatus(proc.StatusEvicted))

	return nil
}

func (nd *Named) newSrv() (*sp.Tendpoint, error) {
	ip := sp.NO_IP
	root := rootDir(nd.fs, nd.realm)
	var addr *sp.Taddr
	var pi portclnt.PortInfo
	if nd.realm == sp.ROOTREALM || nd.ProcEnv().GetNet() == sp.ROOTREALM.String() {
		addr = sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, sp.NO_PORT)
	} else {
		_, pi0, err := portclnt.NewPortClntPort(nd.SigmaClnt.FsLib)
		if err != nil {
			return sp.NewNullEndpoint(), err
		}
		pi = pi0
		addr = sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, pi.PBinding.RealmPort)
	}
	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, nd.SigmaClnt),
		nd.masterPublicKey,
		nd.masterPrivKey,
		nd.signer,
		nd.pubkey,
		nd.privkey,
		//		sp.Tsigner(nd.SigmaClnt.ProcEnv().GetKernelID()),
		//		nd.masterPublicKey,
		//		nil,
	)
	kmgr.AddPublicKey(sp.Tsigner(nd.SigmaClnt.ProcEnv().GetKernelID()), nd.masterPublicKey)
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, nd.signer, sp.NOT_SET, kmgr)
	if err != nil {
		db.DPrintf(db.ERROR, "Error New authsrv: %v", err)
		return sp.NewNullEndpoint(), fmt.Errorf("NewAuthSrv err: %v", err)
	}
	nd.SigmaClnt.SetAuthSrv(as)
	ssrv, err := sigmasrv.NewSigmaSrvRootClntKeyMgr(root, addr, "", nd.SigmaClnt, kmgr)
	if err != nil {
		return sp.NewNullEndpoint(), fmt.Errorf("NewSigmaSrvRootClnt err: %v", err)
	}

	if err := ssrv.MountRPCSrv(newLeaseSrv(nd.fs)); err != nil {
		return sp.NewNullEndpoint(), err
	}
	nd.SigmaSrv = ssrv

	ep := nd.GetEndpoint()
	if nd.realm != sp.ROOTREALM {
		ep = port.NewPublicEndpoint(pi.HostIP, pi.PBinding, nd.ProcEnv().GetNet(), nd.GetEndpoint())
	}
	db.DPrintf(db.NAMED, "newSrv %v %v %v %v %v\n", nd.realm, addr, ssrv.GetEndpoint(), nd.elect.Key(), ep)
	return ep, nil
}

func (nd *Named) attach(cid sp.TclntId) {
	db.DPrintf(db.NAMED, "named: attach %v\n", cid)
	// nd.fs.Recover(cid)
}

func (nd *Named) detach(cid sp.TclntId) {
	db.DPrintf(db.NAMED, "named: detach %v\n", cid)
	// nd.fs.Detach(cid)
}

func (nd *Named) resign() error {
	if err := nd.SigmaPSrv.StopServing(); err != nil {
		return err
	}
	return nd.elect.Resign()
}

func (nd *Named) getRoot(pn string) error {
	sts, err := nd.GetDir(pn)
	if err != nil {
		db.DPrintf(db.NAMED, "getdir %v err %v\n", pn, err)
		return err
	}
	db.DPrintf(db.NAMED, "getdir %v sts %v\n", pn, sp.Names(sts))
	return nil
}

func (nd *Named) waitExit(ch chan struct{}) {
	for {
		err := nd.WaitEvict(nd.ProcEnv().GetPID())
		if err == nil {
			db.DPrintf(db.ALWAYS, "candidate %v %v evicted\n", nd.realm, nd.ProcEnv().GetPID().String())
			ch <- struct{}{}
			break
		}
		db.DPrintf(db.NAMED, "Error WaitEvict: %v", err)
		time.Sleep(time.Second)
		continue
	}
}
