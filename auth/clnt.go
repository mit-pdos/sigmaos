package auth

import (
	db "sigmaos/debug"
)

type AuthClnt struct {
}

func NewAuthClnt() *AuthClnt {
	return &AuthClnt{}
}

func (ac *AuthClnt) GetToken() {
}
