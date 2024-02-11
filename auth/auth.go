package auth

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type SymmetricKey []byte

type GetKeyFn func(signer sp.Tsigner) (SymmetricKey, error)

type KeyMgr interface {
	GetPublicKey(s sp.Tsigner) (SymmetricKey, error)
	GetPrivateKey(s sp.Tsigner) (SymmetricKey, error)
	AddKey(s sp.Tsigner, key SymmetricKey)
}

type AuthSrv interface {
	SetDelegatedProcToken(p *proc.Proc) error
	NewToken(pc *ProcClaims) (*sp.Ttoken, error)
	VerifyTokenGetClaims(signedToken *sp.Ttoken) (*ProcClaims, error)
	IsAuthorized(principal *sp.Tprincipal) (*ProcClaims, bool, error)
	KeyMgr
}

func (sk SymmetricKey) String() string {
	return string(sk)
}
