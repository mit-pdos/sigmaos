package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type AuthMgrImpl[M jwt.SigningMethod] struct {
	signingMethod M
	signer        sp.Tsigner
	srvpath       string
	KeyMgr
}

func NewAuthMgr[M jwt.SigningMethod](method M, signer sp.Tsigner, srvpath string, kmgr KeyMgr) (AuthMgr, error) {
	return &AuthMgrImpl[M]{
		signingMethod: method,
		signer:        signer,
		srvpath:       srvpath,
		KeyMgr:        kmgr,
	}, nil
}

func (as *AuthMgrImpl[M]) GetSrvPath() string {
	return as.srvpath
}

func (as *AuthMgrImpl[M]) MintEndpointToken(ep *sp.Tendpoint) (*sp.Ttoken, error) {
	mc := NewEndpointClaims(ep)
	return as.mintTokenWithClaims(mc)
}

func (as *AuthMgrImpl[M]) MintAndSetEndpointToken(ep *sp.Tendpoint) error {
	token, err := as.MintEndpointToken(ep)
	if err != nil {
		db.DPrintf(db.ERROR, "Error MintEndpointToken: %v", err)
	}
	ep.SetToken(token)
	return nil
}

func (as *AuthMgrImpl[M]) VerifyEndpointTokenGetClaims(principalID sp.TprincipalID, t *sp.Ttoken) (*EndpointClaims, error) {
	claims, err := as.verifyTokenGetClaims(principalID, &EndpointClaims{}, t)
	if err != nil {
		return nil, err
	}
	if mclaims, ok := claims.(*EndpointClaims); ok {
		return mclaims, nil
	}
	return nil, fmt.Errorf("Claims wrong type: %T", claims)
}

func (as *AuthMgrImpl[M]) EndpointIsAuthorized(principal *sp.Tprincipal, endpoint *sp.Tendpoint) (bool, error) {
	db.DPrintf(db.AUTH, "Endpoint Authorization check p %v ep %v", principal.GetID(), endpoint)
	return true, nil
}

// Mint a token with associated claims
func (as *AuthMgrImpl[M]) mintTokenWithClaims(claims jwt.Claims) (*sp.Ttoken, error) {
	privkey, err := as.GetPrivateKey(as.signer)
	if err != nil {
		return nil, err
	}
	// Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt#example-New-Hmac
	token := jwt.NewWithClaims(as.signingMethod, claims)
	tstr, err := token.SignedString(privkey.KeyI())
	if err != nil {
		return nil, err
	}
	return sp.NewToken(as.signer, tstr), err
}

func (as *AuthMgrImpl[M]) verifyTokenGetClaims(principalID sp.TprincipalID, c jwt.Claims, t *sp.Ttoken) (jwt.Claims, error) {
	if t.GetSignedToken() == sp.NO_SIGNED_TOKEN {
		db.DPrintf(db.ERROR, "Tried to verify token when no signed token provided")
		return nil, fmt.Errorf("No signed token provided")
	}
	// Parse the jwt, passing in a function to look up the key.
	//
	// Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt
	token, err := jwt.ParseWithClaims(t.GetSignedToken(), c, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is expected
		if _, ok := token.Method.(M); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		pubkey, err := as.GetPublicKey(t.GetSigner())
		if err != nil {
			return nil, err
		}
		// hmacKey is a []byte containing your secret, e.g. []byte("my_secret_key")
		return pubkey.KeyI(), nil
	})
	if err != nil {
		db.DPrintf(db.ERROR, "Error parsing jwt for principal %v: jwt %v err %v", principalID, t.GetSignedToken(), err)
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("Invalid token")
	}
	return token.Claims, nil
}
