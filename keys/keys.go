package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"
	"path"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

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
	encPrivBytes := make([]byte, base64.StdEncoding.EncodedLen(len(privBytes)))
	base64.StdEncoding.Encode(encPrivBytes, privBytes)
	return encPubBytes, encPrivBytes, nil
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
	return auth.PrivateKey(key), nil
}

func keyPath(s sp.Tsigner) string {
	return path.Join(sp.KEYS, s.String())
}

func PostPublicKey(sc *sigmaclnt.SigmaClnt, s sp.Tsigner, key auth.PublicKey) error {
	// Post the signer's symmetric key in a file
	n, err := sc.PutFile(keyPath(s), 0777, sp.OWRITE, key)
	if err != nil || int(n) != len(key) {
		db.DPrintf(db.ERROR, "Error post key: %v n1 %v n2 %v", err, n, len(key))
		return err
	}
	return nil
}
