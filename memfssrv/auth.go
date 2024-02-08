package memfssrv

import (
	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type HMACVerificationSrv struct {
	sc *sigmaclnt.SigmaClnt
	auth.AuthSrv
}

func NewHMACVerificationSrvKey(signer sp.Tsigner, srvpath string, sc *sigmaclnt.SigmaClnt, key auth.SymmetricKey) (*HMACVerificationSrv, error) {
	as, err := auth.NewHMACAuthSrv(signer, srvpath, key)
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
	// Mount the master key file, which should be mountable by anyone
	key, err := sc.GetFile(sp.MASTER_KEY)
	if err != nil {
		db.DPrintf(db.ERROR, "Error get master key: %v", err)
		return nil, err
	}
	return NewHMACVerificationSrvKey(signer, srvpath, sc, auth.SymmetricKey(key))
}
