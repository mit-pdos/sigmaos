package main

import (
	"os"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/keysrv"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v masterPublicKey masterPrivKey", os.Args[0])
	}
	masterPubKey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(os.Args[1]))
	if err != nil {
		db.DFatalf("Error NewPublicKey", err)
	}
	masterPrivKey, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(os.Args[2]))
	if err != nil {
		db.DFatalf("Error NewPublicKey", err)
	}
	keysrv.RunKeySrv(masterPubKey, masterPrivKey)
}
