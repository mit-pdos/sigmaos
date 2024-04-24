package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"

	sp "sigmaos/sigmap"
)

type EndpointClaims struct {
	Realm sp.Trealm   `json:"realm"`
	Addr  []*sp.Taddr `json:"addr"`
	jwt.StandardClaims
}

// Construct endpoint claims from a endpoint
func NewEndpointClaims(mnt *sp.Tendpoint) *EndpointClaims {
	return &EndpointClaims{
		Realm: mnt.GetRealm(),
		Addr:  mnt.Addrs(),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 999).Unix(), // TODO: set expiry properly
			Issuer:    ISSUER,
		},
	}
}

func (mc *EndpointClaims) String() string {
	return fmt.Sprintf("&{ Realm:%v AllowedPaths:%v }", mc.Realm, mc.Addr)
}
