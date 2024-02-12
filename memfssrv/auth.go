package memfssrv

import (
	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type VerificationSrv struct {
	sc *sigmaclnt.SigmaClnt
	auth.AuthSrv
}

func NewVerificationSrvKeyMgr(signer sp.Tsigner, srvpath string, sc *sigmaclnt.SigmaClnt, kmgr auth.KeyMgr) (*VerificationSrv, error) {
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, signer, srvpath, kmgr)
	if err != nil {
		db.DPrintf(db.ERROR, "Error make auth server: %v", err)
		return nil, err
	}
	return &VerificationSrv{
		sc:      sc,
		AuthSrv: as,
	}, nil
}

func NewVerificationSrv(signer sp.Tsigner, srvpath string, sc *sigmaclnt.SigmaClnt) (*VerificationSrv, error) {
	kmgr := keys.NewKeyMgr(keys.WithSigmaClntGetKeyFn(sc))
	return NewVerificationSrvKeyMgr(signer, srvpath, sc, kmgr)
}
