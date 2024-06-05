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
func NewEndpointClaims(ep *sp.Tendpoint) *EndpointClaims {
	return &EndpointClaims{
		Realm: ep.GetRealm(),
		Addr:  ep.Addrs(),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 999).Unix(), // TODO: set expiry properly
			Issuer:    ISSUER,
		},
	}
}

func (mc *EndpointClaims) String() string {
	return fmt.Sprintf("&{ Realm:%v AllowedPaths:%v }", mc.Realm, mc.Addr)
}
