package memfssrv

import (
	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func WithSigmaClntGetKeyFn(sc *sigmaclnt.SigmaClnt) auth.GetKeyFn {
	return func(signer sp.Tsigner) (auth.SymmetricKey, error) {
		// TODO: use signer
		// Mount the master key file, which should be mountable by anyone
		key, err := sc.GetFile(sp.MASTER_KEY)
		if err != nil {
			db.DPrintf(db.ERROR, "Error get master key: %v", err)
			return nil, err
		}
		return auth.SymmetricKey(key), nil
	}
}

type HMACVerificationSrv struct {
	sc *sigmaclnt.SigmaClnt
	auth.AuthSrv
}

func NewHMACVerificationSrvKeyMgr(signer sp.Tsigner, srvpath string, sc *sigmaclnt.SigmaClnt, kmgr *auth.KeyMgr) (*HMACVerificationSrv, error) {
	as, err := auth.NewHMACAuthSrv(signer, srvpath, kmgr)
	if err != nil {
		db.DPrintf(db.ERROR, "Error make auth server: %v", err)
		return nil, err
	}
	return &HMACVerificationSrv{
		sc:      sc,
		AuthSrv: as,
	}, nil
}

func NewHMACVerificationSrv(signer sp.Tsigner, srvpath string, sc *sigmaclnt.SigmaClnt) (*HMACVerificationSrv, error) {
	kmgr := auth.NewKeyMgr(WithSigmaClntGetKeyFn(sc))
	return NewHMACVerificationSrvKeyMgr(signer, srvpath, sc, kmgr)
}
