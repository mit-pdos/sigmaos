package auth

type AuthSrv interface {
	NewToken(claims *ProcClaims) (string, error)
	VerifyTokenGetClaims(signedToken string) (*ProcClaims, error)
}
