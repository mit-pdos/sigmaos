package auth

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type KeyMgr interface {
	GetPublicKey(s sp.Tsigner) (PublicKey, error)
	GetPrivateKey(s sp.Tsigner) (PrivateKey, error)
	AddPublicKey(s sp.Tsigner, key PublicKey)
	AddPrivateKey(s sp.Tsigner, key PrivateKey)
}

type AuthSrv interface {
	SetDelegatedProcToken(p *proc.Proc) error
	// Mount tokens
	//	MintMountToken(mnt *MountClaims) (*sp.Ttoken, error)
	//	MintAndSetMountToken(mnt *sp.Tmount) error
	//	VerifyMountTokenGetClaims(principalID sp.TprincipalID, signedMountToken *sp.Ttoken) (*MountClaims, error)
	//	MountIsAuthorized(principal *sp.Tprincipal, attachPath string) (*MountClaims, bool, error)
	// Proc tokens
	MintProcToken(pc *ProcClaims) (*sp.Ttoken, error)
	MintAndSetProcToken(pe *proc.ProcEnv) error
	VerifyProcTokenGetClaims(principalID sp.TprincipalID, signedProcToken *sp.Ttoken) (*ProcClaims, error)
	AttachIsAuthorized(principal *sp.Tprincipal, attachPath string) (*ProcClaims, bool, error)
	KeyMgr
}
