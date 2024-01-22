package auth

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type AuthSrv struct {
}

func NewAuthSrv() *AuthSrv {
	return &AuthSrv{}
}

func (as *AuthSrv) IsAuthorized(principal *sp.Tprincipal) bool {
	db.DPrintf(db.AUTH, "Authorization check p %v", principal)
	// TODO: do a real check
	if principal.TokenPresent {
		db.DPrintf(db.AUTH, "Authorization check successful p %v", principal)
		return true
	}
	db.DPrintf(db.AUTH, "Authorization check failed p %v", principal)
	return false
}
