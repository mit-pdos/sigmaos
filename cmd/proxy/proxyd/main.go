package main

import (
	"os"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/keys"
	"sigmaos/netsrv"
	"sigmaos/npcodec"
	"sigmaos/proc"
	"sigmaos/proxy"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("%s: Usage <lip>\n", os.Args[0])
	}
	lip := sp.Tip(os.Args[1])
	s3secrets, err1 := auth.GetAWSSecrets()
	if err1 != nil {
		db.DFatalf("Failed to load AWS secrets %v", err1)
	}
	secrets := map[string]*proc.ProcSecretProto{"s3": s3secrets}
	// By default, proxy doesn't use overlays.
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, lip, lip, lip, "", false, false)
	pe.Program = "proxy"
	pe.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID("proxy"),
		sp.NoToken(),
	))
	masterKey, err1 := os.ReadFile(sp.HOST_KEY_FILE)
	if err1 != nil {
		db.DFatalf("Error Read master key: %v", err1)
	}
	kmgr := keys.NewKeyMgr(keys.WithConstGetKeyFn(masterKey))
	kmgr.AddPublicKey(auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, masterKey)
	as, err1 := auth.NewAuthSrv[*jwt.SigningMethodHMAC](jwt.SigningMethodHS256, auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, proc.NOT_SET, kmgr)
	if err1 != nil {
		db.DFatalf("Error NewAuthSrv: %v", err1)
	}
	pc := auth.NewProcClaims(pe)
	token, err1 := as.NewToken(pc)
	if err1 != nil {
		db.DFatalf("Error NewToken: %v", err1)
	}
	pe.SetToken(token)
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	netsrv.NewNetServer(pe, proxy.NewNpd(pe, lip), addr, npcodec.ReadCall, npcodec.WriteCall)
	ch := make(chan struct{})
	<-ch
}
