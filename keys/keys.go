package keys

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func NewSymmetricKey(nbyte int) (auth.SymmetricKey, error) {
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
	return auth.SymmetricKey(key), nil
}

func keyPath(s sp.Tsigner) string {
	return path.Join(sp.KEYS, s.String())
}

func PostSymmetricKey(sc *sigmaclnt.SigmaClnt, s sp.Tsigner, key auth.SymmetricKey) error {
	// Post the signer's symmetric key in a file
	n, err := sc.PutFile(keyPath(s), 0777, sp.OWRITE, key)
	if err != nil || int(n) != len(key) {
		db.DPrintf(db.ERROR, "Error post key: %v n1 %v n2 %v", err, n, len(key))
		return err
	}
	return nil
}
