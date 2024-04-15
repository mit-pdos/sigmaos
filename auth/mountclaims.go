package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"

	sp "sigmaos/sigmap"
)

type MountClaims struct {
	Realm sp.Trealm   `json:"realm"`
	Addr  []*sp.Taddr `json:"addr"`
	jwt.StandardClaims
}

// Construct mount claims from a mount
func NewMountClaims(mnt *sp.Tmount) *MountClaims {
	return &MountClaims{
		Realm: mnt.GetRealm(),
		Addr:  mnt.Addrs(),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 999).Unix(), // TODO: set expiry properly
			Issuer:    ISSUER,
		},
	}
}

func (mc *MountClaims) String() string {
	return fmt.Sprintf("&{ Realm:%v AllowedPaths:%v }", mc.Realm, mc.Addr)
}
