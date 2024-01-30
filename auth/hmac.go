package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type HMACAuthSrv struct {
	srvpath    string
	hmacSecret []byte
}

func NewHMACAuthSrv(srvpath string, hmacSecret SigVerificationKey) (*HMACAuthSrv, error) {
	return &HMACAuthSrv{
		srvpath:    srvpath,
		hmacSecret: []byte(hmacSecret),
	}, nil
}

// Set a proc's token after it has been spawned by the parent
func (as *HMACAuthSrv) SetDelegatedProcToken(p *proc.Proc) error {
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

func (as *HMACAuthSrv) NewToken(pc *ProcClaims) (string, error) {
	// Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt#example-New-Hmac
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, pc)
	return token.SignedString(as.hmacSecret)
}

func (as *HMACAuthSrv) VerifyTokenGetClaims(signedToken string) (*ProcClaims, error) {
	// Parse the jwt, passing in a function to look up the key.
	//
	// Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt#example-Parse-Hmac
	token, err := jwt.ParseWithClaims(signedToken, &ProcClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is expected
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		// hmacSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return as.hmacSecret, nil
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

func (as *HMACAuthSrv) IsAuthorized(principal *sp.Tprincipal) (*ProcClaims, bool, error) {
	db.DPrintf(db.AUTH, "Authorization check p %v", principal.ID)
	pc, err := as.VerifyTokenGetClaims(principal.TokenStr)
	if err != nil {
		db.DPrintf(db.AUTH, "Token verification failed %v", principal.ID)
		return nil, false, fmt.Errorf("Token verification failed: %v", err)
	}
	// Check that the server path is a subpath of one of the allowed paths
	for _, ap := range pc.AllowedPaths {
		db.DPrintf(db.AUTH, "Check if %v is in %v subtree", as.srvpath, ap)
		if as.srvpath == "" && ap == sp.NAMED {
			db.DPrintf(db.AUTH, "Authorization check to named successful p %v claims %v", principal.ID, pc)
			return pc, true, nil
		}
		if IsInSubtree(as.srvpath, ap) {
			db.DPrintf(db.AUTH, "Authorization check successful p %v claims %v", principal.ID, pc)
			return pc, true, nil
		}
	}
	db.DPrintf(db.AUTH, "Authorization check failed (path not allowed) srvpath %v p %v claims %v", as.srvpath, principal.ID, pc)
	return nil, false, nil
}
