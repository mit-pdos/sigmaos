package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

func LoadMasterECDSAKey() (auth.PublicKey, auth.PrivateKey, error) {
	masterPrivKeyBytes, err := os.ReadFile(sp.HOST_PRIV_KEY_FILE)
	if err != nil {
		db.DPrintf(db.ERROR, "Error Read master private key: %v", err)
		return nil, nil, err
	}
	masterPubKeyBytes, err := os.ReadFile(sp.HOST_PUB_KEY_FILE)
	if err != nil {
		db.DPrintf(db.ERROR, "Error Read master private key: %v", err)
		return nil, nil, err
	}
	masterPubKey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, masterPubKeyBytes)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewPublicKey: %v", err)
		return nil, nil, err
	}
	masterPrivKey, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, masterPrivKeyBytes)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewPrivateKey: %v", err)
		return nil, nil, err
	}
	return masterPubKey, masterPrivKey, nil
}

func NewECDSAKey() (auth.PublicKey, auth.PrivateKey, error) {
	privkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		db.DPrintf(db.ERROR, "Error generate key: %v", err)
		return nil, nil, err
	}
	pubkey := privkey.Public()
	privBytes, err := x509.MarshalECPrivateKey(privkey)
	if err != nil {
		db.DPrintf(db.ERROR, "Error marshal privkey: %v", err)
		return nil, nil, err
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(pubkey)
	if err != nil {
		db.DPrintf(db.ERROR, "Error marshal pubkey: %v", err)
		return nil, nil, err
	}
	encPubBytes := make([]byte, base64.StdEncoding.EncodedLen(len(pubBytes)))
	base64.StdEncoding.Encode(encPubBytes, pubBytes)
	pk, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, encPubBytes)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewPublicKey: %v", err)
		return nil, nil, err
	}
	encPrivBytes := make([]byte, base64.StdEncoding.EncodedLen(len(privBytes)))
	base64.StdEncoding.Encode(encPrivBytes, privBytes)
	sk, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, encPrivBytes)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewPrivateKey: %v", err)
		return nil, nil, err
	}
	return pk, sk, nil
}

func NewSymmetricKey(nbyte int) (auth.PrivateKey, error) {
	file, err := os.Open("/dev/urandom") // For read access.
	if err != nil {
		db.DPrintf(db.ERROR, "Error open /dev/urandom: %v", err)
		return nil, err
	}
	randBytes := make([]byte, nbyte)
	n, err := file.Read(randBytes)
	if err != nil {
		db.DPrintf(db.ERROR, "Error read /dev/urandom: err %v", err)
		return nil, err
	}
	if n != nbyte {
		db.DPrintf(db.ERROR, "Didn't read enough bytes /dev/urandom: n %v != %v", n, nbyte)
		return nil, fmt.Errorf("Didn't read enough bytes from /dev/urandom: %v != %v", n, nbyte)
	}
	key := make([]byte, base64.StdEncoding.EncodedLen(len(randBytes)))
	base64.StdEncoding.Encode(key, randBytes)
	return auth.NewPrivateKey[*jwt.SigningMethodHMAC](jwt.SigningMethodHS256, key)
}

func BootstrappedKeysFromArgs(args []string) (auth.PublicKey, auth.PublicKey, auth.PrivateKey, error) {
	if len(args) < 3 {
		return nil, nil, nil, fmt.Errorf("Too few args for keys: %v", len(args))
	}
	masterPubKey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(args[0]))
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewPublicKey", err)
		return nil, nil, nil, err
	}
	pubkey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(args[1]))
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewPublicKey", err)
		return nil, nil, nil, err
	}
	privkey, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(args[2]))
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewPrivateKey", err)
		return nil, nil, nil, err
	}
	return masterPubKey, pubkey, privkey, nil
}
