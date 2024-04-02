package auth

import (
	"fmt"
	"path"

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
	parentPC, err := as.VerifyProcTokenGetClaims(p.GetPrincipal().GetID(), p.GetParentToken())
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
	// If this proc already contains a valid signed token (presumably signed by
	// the kernelsrv during spawn), then bail out. No need to re-sign. What's
	// more, baliing out is important for bootstrapping, because the keyd server
	// will only know about the kernel srv's key during bootstrapping, not
	// schedd's key.
	if p.GetPrincipal().GetToken().GetSignedToken() != sp.NO_SIGNED_TOKEN {
		return nil
	}
	// Parent's token is valid, and child's token only contains allowed paths
	// which are a subset of the parent's. Sign the child's token.
	token, err := as.MintProcToken(pc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error MintToken: %v", err)
		db.DPrintf(db.AUTH, "Error MintToken: %v", err)
		return err
	}
	p.SetToken(token)
	return nil
}

func (as *AuthSrvImpl[M]) MintMountToken(mnt *sp.Tmount) (*sp.Ttoken, error) {
	mc := NewMountClaims(mnt)
	return as.mintTokenWithClaims(mc)
}

func (as *AuthSrvImpl[M]) MintAndSetMountToken(mnt *sp.Tmount) error {
	token, err := as.MintMountToken(mnt)
	if err != nil {
		db.DPrintf(db.ERROR, "Error MintMountToken: %v", err)
	}
	mnt.SetToken(token)
	return nil
}

func (as *AuthSrvImpl[M]) VerifyMountTokenGetClaims(principalID sp.TprincipalID, t *sp.Ttoken) (*MountClaims, error) {
	claims, err := as.verifyTokenGetClaims(principalID, t)
	if err != nil {
		return nil, err
	}
	if mclaims, ok := claims.(*MountClaims); ok {
		return mclaims, nil
	}
	return nil, fmt.Errorf("Claims wrong type")
}

func (as *AuthSrvImpl[M]) MintAndSetProcToken(pe *proc.ProcEnv) error {
	pc := NewProcClaims(pe)
	token, err := as.MintProcToken(pc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error MintToken: %v", err)
		return err
	}
	pe.SetToken(token)
	return nil
}

func (as *AuthSrvImpl[M]) MintProcToken(pc *ProcClaims) (*sp.Ttoken, error) {
	return as.mintTokenWithClaims(pc)
}

func (as *AuthSrvImpl[M]) VerifyProcTokenGetClaims(principalID sp.TprincipalID, t *sp.Ttoken) (*ProcClaims, error) {
	claims, err := as.verifyTokenGetClaims(principalID, t)
	if err != nil {
		return nil, err
	}
	if pclaims, ok := claims.(*ProcClaims); ok {
		return pclaims, nil
	}
	return nil, fmt.Errorf("Claims wrong type")
}

func (as *AuthSrvImpl[M]) verifyPrincipalIdentity(principal *sp.Tprincipal) (*ProcClaims, error) {
	db.DPrintf(db.AUTH, "Verify ID p %v", principal.GetID())
	pc, err := as.VerifyProcTokenGetClaims(principal.GetID(), principal.GetToken())
	if err != nil {
		db.DPrintf(db.AUTH, "ID token verification failed %v", principal.GetID())
		return nil, fmt.Errorf("Token verification failed: %v", err)
	}
	if principal.GetID() != pc.PrincipalID {
		db.DPrintf(db.AUTH, "ID verification failed p %v, Token & principal ID don't match ( %v != %v )", principal.GetID(), principal.GetID(), pc.PrincipalID)
		return nil, fmt.Errorf("Mismatch between principal ID and token ID: %v", err)
	}
	if principal.GetRealm() != pc.Realm {
		db.DPrintf(db.AUTH, "ID verification failed p %v, Token & realm ID don't match ( %v != %v )", principal.GetID(), principal.GetRealm(), pc.Realm)
		return nil, fmt.Errorf("Mismatch between realm ID and token ID: %v", err)
	}
	return pc, nil
}

func (as *AuthSrvImpl[M]) AttachIsAuthorized(principal *sp.Tprincipal, attachPath string) (*ProcClaims, bool, error) {
	db.DPrintf(db.AUTH, "Attach Authorization check p %v", principal.GetID())
	pc, err := as.verifyPrincipalIdentity(principal)
	if err != nil {
		db.DPrintf(db.AUTH, "Attach Authorization check failed p %v: err %v", principal.GetID(), err)
		return nil, false, err
	}
	// Check that the server path is a subpath of one of the allowed paths
	for _, ap := range pc.AllowedPaths {
		mntPath := path.Join(as.srvpath, attachPath)
		db.DPrintf(db.AUTH, "Check if %v or %v is in %v subtree", as.srvpath, mntPath, ap)
		if as.srvpath == "" && ap == sp.NAMED {
			db.DPrintf(db.AUTH, "Attach Authorization check to named successful p %v claims %v", principal.GetID(), pc)
			return pc, true, nil
		}
		if IsInSubtree(as.srvpath, ap) || IsInSubtree(mntPath, ap) {
			db.DPrintf(db.AUTH, "Attach Authorization check successful p %v claims %v", principal.GetID(), pc)
			return pc, true, nil
		}
	}
	db.DPrintf(db.AUTH, "Attach Authorization check failed (path not allowed) srvpath %v p %v claims %v", as.srvpath, principal.GetID(), pc)
	return nil, false, nil
}

func (as *AuthSrvImpl[M]) MountIsAuthorized(principal *sp.Tprincipal, mount *sp.Tmount) (bool, error) {
	db.DPrintf(db.AUTH, "Mount Authorization check p %v", principal.GetID())
	pc, err := as.verifyPrincipalIdentity(principal)
	if err != nil {
		db.DPrintf(db.AUTH, "Mount Authorization check failed p %v: err %v", principal.GetID(), err)
		return false, err
	}
	// Check if the mount is for the principal's realm
	if pc.Realm != mount.GetRealm() {
		err := fmt.Errorf("Mismatch between p %v realm %v and mount %v realm %v", principal.GetID(), pc.Realm, mount, mount.GetRealm())
		db.DPrintf(db.AUTH, "Mount Authorization check failed p %v: err %v", principal.GetID(), err)
		return false, err
	}
	return true, nil
}

// Mint a token with associated claims
func (as *AuthSrvImpl[M]) mintTokenWithClaims(claims jwt.Claims) (*sp.Ttoken, error) {
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

func (as *AuthSrvImpl[M]) verifyTokenGetClaims(principalID sp.TprincipalID, t *sp.Ttoken) (jwt.Claims, error) {
	if t.GetSignedToken() == sp.NO_SIGNED_TOKEN {
		db.DPrintf(db.ERROR, "Tried to veryify token when no signed token provided")
		return nil, fmt.Errorf("No signed token provided")
	}
	// Parse the jwt, passing in a function to look up the key.
	//
	// Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt
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
