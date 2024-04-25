package dbsrv

import (
	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/proc"
	"sigmaos/sessdevsrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

//
// A db proxy exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

const (
	QDEV = "query"
)

func RunDbd(dbdaddr string, masterPubKey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) error {
	s, err := newServer(dbdaddr)
	if err != nil {
		return err
	}
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
		return err
	}
	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc),
		masterPubKey,
		nil,
		sp.Tsigner(pe.GetPID()),
		pubkey,
		privkey,
	)
	amgr, err := auth.NewAuthMgr[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf("Error NewAuthMgr %v", err)
	}
	sc.SetAuthMgr(amgr)

	ssrv, err := sigmasrv.NewSigmaSrvClnt(sp.DB, sc, s)
	if err != nil {
		return err
	}
	qd := &queryDev{dbdaddr}
	if _, err := ssrv.Create(QDEV, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := sessdevsrv.NewSessDev(ssrv.MemFs, QDEV, qd.newSession, nil); err != nil {
		return err
	}
	return ssrv.RunServer()
}
