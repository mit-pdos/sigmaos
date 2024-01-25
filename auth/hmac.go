package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type HMACAuthSrv struct {
	hmacSecret []byte
}

func NewHMACAuthSrv(hmacSecret []byte) (*HMACAuthSrv, error) {
	return &HMACAuthSrv{
		hmacSecret: hmacSecret,
	}, nil
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

func (as *HMACAuthSrv) IsAuthorized(principal *sp.Tprincipal) (bool, error) {
	db.DPrintf(db.AUTH, "Authorization check p %v", principal)
	pc, err := as.VerifyTokenGetClaims(principal.TokenStr)
	if err != nil {
		db.DPrintf(db.AUTH, "Token verification failed %v", principal)
		return false, fmt.Errorf("Token verification failed: %v", err)
	}
	db.DPrintf(db.AUTH, "Authorization check successful p %v claims %v", principal, pc)
	// TODO: check paths in claims
	//db.DPrintf(db.AUTH, "Authorization check failed p %v", principal)
	return true, nil
}
