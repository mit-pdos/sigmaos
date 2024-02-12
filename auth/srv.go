package auth

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"github.com/golang-jwt/jwt"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type AuthSrvImpl[M jwt.SigningMethod] struct {
	signingMethod M
	signer        sp.Tsigner
	srvpath       string
	KeyMgr
}

func NewAuthSrv[M jwt.SigningMethod](method M, signer sp.Tsigner, srvpath string, kmgr KeyMgr) (AuthSrv, error) {
	return &AuthSrvImpl[M]{
		signingMethod: method,
		signer:        signer,
		srvpath:       srvpath,
		KeyMgr:        kmgr,
	}, nil
}

func (as *AuthSrvImpl[M]) GetSrvPath() string {
	return as.srvpath
}

// Set a proc's token after it has been spawned by the parent
func (as *AuthSrvImpl[M]) SetDelegatedProcToken(p *proc.Proc) error {
	// Retrieve and validate the proc's parent's claims
	parentPC, err := as.VerifyTokenGetClaims(p.GetParentToken())
	if err != nil {
		db.DPrintf(db.ERROR, "Error verify parent token: %v", err)
		db.DPrintf(db.AUTH, "Error verify parent token: %v", err)
		return err
	}
	// Retrieve the proc's claims
	pc := NewProcClaims(p.GetProcEnv())
	// Ensure the child proc's allowed paths are a subset of the parent proc's
	// allowed paths
	for _, ap := range pc.AllowedPaths {
		subset := false
		for _, parentAP := range parentPC.AllowedPaths {
			// If the child path is a subset of one of the parent's allowed paths,
			// stop iterating
			if IsInSubtree(ap, parentAP) {
				subset = true
				break
			}
		}
		if !subset {
			db.DPrintf(db.ERROR, "Child's allowed paths not a subset of parent's: p %v c %v", ap, parentPC.AllowedPaths)
			db.DPrintf(db.AUTH, "Child's allowed paths not a subset of parent's: p %v c %v", ap, parentPC.AllowedPaths)
			return fmt.Errorf("Child's allowed paths not a subset of parent's: p %v c %v", ap, parentPC.AllowedPaths)
		}
	}
	// Parent's token is valid, and child's token only contains allowed paths
	// which are a subset of the parent's. Sign the child's token.
	token, err := as.NewToken(pc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewToken: %v", err)
		db.DPrintf(db.AUTH, "Error NewToken: %v", err)
		return err
	}
	p.SetToken(token)
	return nil
}

func (as *AuthSrvImpl[M]) NewToken(pc *ProcClaims) (*sp.Ttoken, error) {
	privkey, err := as.GetPrivateKey(as.signer)
	if err != nil {
		return nil, err
	}
	// Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt#example-New-Hmac
	token := jwt.NewWithClaims(as.signingMethod, pc)
	var key interface{}
	switch any(as.signingMethod).(type) {
	case *jwt.SigningMethodECDSA:
		b, err := base64.StdEncoding.DecodeString(string(privkey))
		key, err = x509.ParseECPrivateKey(b)
		if err != nil {
			return nil, err
		}
	case *jwt.SigningMethodHMAC:
		key = []byte(privkey)
	}
	tstr, err := token.SignedString(key)
	if err != nil {
		return nil, err
	}
	return sp.NewToken(as.signer, tstr), err
}

func (as *AuthSrvImpl[M]) VerifyTokenGetClaims(t *sp.Ttoken) (*ProcClaims, error) {
	// Parse the jwt, passing in a function to look up the key.
	//
	// Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt#example-Parse-Hmac
	token, err := jwt.ParseWithClaims(t.GetSignedToken(), &ProcClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is expected
		if _, ok := token.Method.(M); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		pubkey, err := as.GetPublicKey(t.GetSigner())
		if err != nil {
			return nil, err
		}
		// hmacKey is a []byte containing your secret, e.g. []byte("my_secret_key")
		var key interface{}
		switch any(as.signingMethod).(type) {
		case *jwt.SigningMethodECDSA:
			b, err := base64.StdEncoding.DecodeString(string(pubkey))
			key, err = x509.ParsePKIXPublicKey(b)
			if err != nil {
				return nil, err
			}
		case *jwt.SigningMethodHMAC:
			key = []byte(pubkey)
		}
		return key, nil
	})
	if err != nil {
		db.DPrintf(db.ERROR, "Error parsing jwt: %v", err)
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("Invalid token")
	}
	if pclaims, ok := token.Claims.(*ProcClaims); ok {
		return pclaims, nil
	}
	return nil, fmt.Errorf("Claims wrong type")
}

func (as *AuthSrvImpl[M]) IsAuthorized(principal *sp.Tprincipal) (*ProcClaims, bool, error) {
	db.DPrintf(db.AUTH, "Authorization check p %v", principal.GetID())
	pc, err := as.VerifyTokenGetClaims(principal.GetToken())
	if err != nil {
		db.DPrintf(db.AUTH, "Token verification failed %v", principal.GetID())
		db.DPrintf(db.AUTH, "Authorization check failed p %v, Token verification failed", principal.GetID())
		return nil, false, fmt.Errorf("Token verification failed: %v", err)
	}
	if principal.GetID() != pc.PrincipalID {
		db.DPrintf(db.AUTH, "Authorization check failed p %v, Token & principal ID don't match ( %v != %v )", principal.GetID(), principal.GetID(), pc.PrincipalID)
		return nil, false, fmt.Errorf("Mismatch between principal ID and token ID: %v", err)
	}
	// Check that the server path is a subpath of one of the allowed paths
	for _, ap := range pc.AllowedPaths {
		db.DPrintf(db.AUTH, "Check if %v is in %v subtree", as.srvpath, ap)
		if as.srvpath == "" && ap == sp.NAMED {
			db.DPrintf(db.AUTH, "Authorization check to named successful p %v claims %v", principal.GetID(), pc)
			return pc, true, nil
		}
		if IsInSubtree(as.srvpath, ap) {
			db.DPrintf(db.AUTH, "Authorization check successful p %v claims %v", principal.GetID(), pc)
			return pc, true, nil
		}
	}
	db.DPrintf(db.AUTH, "Authorization check failed (path not allowed) srvpath %v p %v claims %v", as.srvpath, principal.GetID(), pc)
	return nil, false, nil
}