package auth

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type SymmetricKey []byte

type GetKeyFn func(signer sp.Tsigner) (SymmetricKey, error)

type AuthSrv interface {
	SetDelegatedProcToken(p *proc.Proc) error
	NewToken(pc *ProcClaims) (*sp.Ttoken, error)
	VerifyTokenGetClaims(signedToken *sp.Ttoken) (*ProcClaims, error)
	IsAuthorized(principal *sp.Tprincipal) (*ProcClaims, bool, error)
}
