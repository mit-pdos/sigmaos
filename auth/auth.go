package auth

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type KeyMgr interface {
	GetPublicKey(s sp.Tsigner) (PublicKey, error)
	GetPrivateKey(s sp.Tsigner) (PrivateKey, error)
	AddPublicKey(s sp.Tsigner, key PublicKey)
	AddPrivateKey(s sp.Tsigner, key PrivateKey)
}

type AuthMgr interface {
	SetDelegatedProcToken(p *proc.Proc) error
	VerifyPrincipalIdentity(principal *sp.Tprincipal) (*ProcClaims, error)
	// Endpoint tokens
	MintEndpointToken(ep *sp.Tendpoint) (*sp.Ttoken, error)
	MintAndSetEndpointToken(ep *sp.Tendpoint) error
	VerifyEndpointTokenGetClaims(principalID sp.TprincipalID, signedEndpointToken *sp.Ttoken) (*EndpointClaims, error)
	EndpointIsAuthorized(principal *sp.Tprincipal, ep *sp.Tendpoint) (bool, error)
	// Proc tokens
	MintProcToken(pc *ProcClaims) (*sp.Ttoken, error)
	MintAndSetProcToken(pe *proc.ProcEnv) error
	VerifyProcTokenGetClaims(principalID sp.TprincipalID, signedProcToken *sp.Ttoken) (*ProcClaims, error)
	AttachIsAuthorized(principal *sp.Tprincipal, attachPath string) (*ProcClaims, bool, error)
	KeyMgr
}
