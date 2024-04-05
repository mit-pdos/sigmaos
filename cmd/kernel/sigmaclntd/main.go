package main

import (
	"os"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/sigmaclntsrv"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v masterPublicKey pubkey privkey", os.Args[0])
	}
	masterPubKey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(os.Args[1]))
	if err != nil {
		db.DFatalf("Error NewPublicKey", err)
	}
	pubkey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(os.Args[2]))
	if err != nil {
		db.DFatalf("Error NewPublicKey", err)
	}
	privkey, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(os.Args[3]))
	if err != nil {
		db.DFatalf("Error NewPrivateKey", err)
	}
	if err := sigmaclntsrv.RunSigmaClntSrv(masterPubKey, pubkey, privkey); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
