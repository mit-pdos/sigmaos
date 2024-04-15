package fsux

import (
	"path"
	"sync"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/keys"
	"sigmaos/netsigma"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	// "sigmaos/seccomp"
)

var fsux *FsUx

type FsUx struct {
	*sigmaclnt.SigmaClnt
	*sigmasrv.SigmaSrv
	mount string

	sync.Mutex
	ot *ObjTable
}

func RunFsUx(rootux string, masterPubKey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) {
	pe := proc.GetProcEnv()
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
		db.DFatalf("Error NewAuthSrv %v", err)
	}
	sc.SetAuthSrv(as)

	ip, err := netsigma.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}
	fsux, root := NewUx(rootux)
	addr := sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, sp.NO_PORT)
	srv, err := sigmasrv.NewSigmaSrvRootClnt(root, path.Join(sp.UX, pe.GetKernelID()), addr, sc)
	if err != nil {
		db.DFatalf("NewSigmaSrvRootClnt %v\n", err)
	}
	fsux.SigmaSrv = srv

	// Perf monitoring
	p, err := perf.NewPerf(pe, perf.UX)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	fsux.RunServer()
}

func NewUx(rootux string) (*FsUx, fs.Dir) {
	fsux = &FsUx{}
	fsux.ot = NewObjTable()
	root, sr := newDir([]string{rootux})
	if sr != nil {
		db.DFatalf("newDir %v\n", sr)
	}
	return fsux, root
}
