package keys

import (
	"encoding/base64"
	"fmt"
	"os"

	"sigmaos/auth"
	db "sigmaos/debug"
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
