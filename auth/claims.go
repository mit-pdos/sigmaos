package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt"
)

type ProcClaims struct {
	PID          string   `json:"pid"`
	PrincipalID  string   `json:"principal_id"`
	AllowedPaths []string `json:"allowed_paths"`
	jwt.StandardClaims
}

func (pc *ProcClaims) String() string {
	return fmt.Sprintf("&{ PID:%v PrincipaLID:%v AllowedPaths:%v }", pc.PID, pc.PrincipalID, pc.AllowedPaths)
}
