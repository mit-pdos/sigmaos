package auth

import (
	sp "sigmaos/sigmap"
)

type AuthSrv interface {
	NewToken(pc *ProcClaims) (string, error)
	VerifyTokenGetClaims(signedToken string) (*ProcClaims, error)
	IsAuthorized(principal *sp.Tprincipal) (bool, error)
}
