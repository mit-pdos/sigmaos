package auth

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type SigVerificationKey []byte

type AuthSrv interface {
	SetDelegatedProcToken(p *proc.Proc) error
	NewToken(pc *ProcClaims) (string, error)
	VerifyTokenGetClaims(signedToken string) (*ProcClaims, error)
	IsAuthorized(principal *sp.Tprincipal) (*ProcClaims, bool, error)
}
