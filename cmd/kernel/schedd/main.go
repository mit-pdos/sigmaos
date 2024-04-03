package main

import (
	"os"
	"strconv"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/schedsrv"
)

func main() {
	if len(os.Args) != 6 {
		db.DFatalf("Usage: %v masterPublicKey pubkey privkey kernelId reserveMcpu", os.Args[0])
	}
	reserveMcpu, err := strconv.ParseUint(os.Args[5], 10, 32)
	if err != nil {
		db.DFatalf("Cannot parse reserve cpu unit \"%v\": %v", os.Args[5], err)
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
	schedsrv.RunSchedd(os.Args[4], uint(reserveMcpu), masterPubKey, pubkey, privkey)
}
