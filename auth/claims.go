package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"

	"sigmaos/proc"
)

const (
	ISSUER = "sigmaos"
)

var ALL_PATHS []string = []string{"*"}

type ProcClaims struct {
	PID string `json:"pid"`
	//	PrincipalID  string   `json:"principal_id"`
	AllowedPaths []string `json:"allowed_paths"`
	jwt.StandardClaims
}

// Construct proc claims from a proc env
func NewProcClaims(pe *proc.ProcEnv) *ProcClaims {
	return &ProcClaims{
		PID:          pe.GetClaims().GetPID().String(),
		AllowedPaths: pe.GetClaims().GetAllowedPaths(),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 999).Unix(), // TODO: set expiry properly
			Issuer:    ISSUER,
		},
	}
}

func (pc *ProcClaims) String() string {
	return fmt.Sprintf("&{ PID:%v AllowedPaths:%v }", pc.PID, pc.AllowedPaths)
}
