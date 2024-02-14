package auth

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type PublicKey []byte
type PrivateKey []byte

type KeyMgr interface {
	GetPublicKey(s sp.Tsigner) (PublicKey, error)
	GetPrivateKey(s sp.Tsigner) (PrivateKey, error)
	AddPublicKey(s sp.Tsigner, key PublicKey)
	AddPrivateKey(s sp.Tsigner, key PrivateKey)
}

type AuthSrv interface {
	SetDelegatedProcToken(p *proc.Proc) error
	MintToken(pc *ProcClaims) (*sp.Ttoken, error)
	VerifyTokenGetClaims(signedToken *sp.Ttoken) (*ProcClaims, error)
	IsAuthorized(principal *sp.Tprincipal) (*ProcClaims, bool, error)
	KeyMgr
}

func (sk PublicKey) String() string {
	return string(sk)
}

func (sk PrivateKey) String() string {
	return string(sk)
}
