package memfssrv

import (
	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type HMACVerificationSrv struct {
	sc *sigmaclnt.SigmaClnt
	auth.AuthSrv
}

func NewHMACVerificationSrvKeyMgr(signer sp.Tsigner, srvpath string, sc *sigmaclnt.SigmaClnt, kmgr auth.KeyMgr) (*HMACVerificationSrv, error) {
	as, err := auth.NewAuthSrv[*jwt.SigningMethodHMAC](jwt.SigningMethodHS256, signer, srvpath, kmgr)
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
	kmgr := keys.NewSymmetricKeyMgr(keys.WithSigmaClntGetKeyFn(sc))
	return NewHMACVerificationSrvKeyMgr(signer, srvpath, sc, kmgr)
}
