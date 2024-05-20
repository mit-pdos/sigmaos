package auth

import (
	sp "sigmaos/sigmap"
)

type KeyMgr interface {
	GetPublicKey(s sp.Tsigner) (PublicKey, error)
	GetPrivateKey(s sp.Tsigner) (PrivateKey, error)
	AddPublicKey(s sp.Tsigner, key PublicKey)
	AddPrivateKey(s sp.Tsigner, key PrivateKey)
}

type AuthMgr interface {
	// Endpoint tokens
	MintEndpointToken(ep *sp.Tendpoint) (*sp.Ttoken, error)
	MintAndSetEndpointToken(ep *sp.Tendpoint) error
	VerifyEndpointTokenGetClaims(principalID sp.TprincipalID, signedEndpointToken *sp.Ttoken) (*EndpointClaims, error)
	EndpointIsAuthorized(principal *sp.Tprincipal, ep *sp.Tendpoint) (bool, error)
	KeyMgr
}
