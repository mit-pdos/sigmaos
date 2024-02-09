package auth

import (
	"encoding/base64"
	"fmt"
	"os"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type KeyMgr struct {
	mu     sync.Mutex
	getKey GetKeyFn
	keys   map[sp.Tsigner]SymmetricKey
}

func NewKeyMgr(fn GetKeyFn) *KeyMgr {
	return &KeyMgr{
		getKey: fn,
		keys:   make(map[sp.Tsigner]SymmetricKey),
	}
}

func (mgr *KeyMgr) GetKey(s sp.Tsigner) (SymmetricKey, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	db.DPrintf(db.AUTH, "Get key for signer %v", s)
	var key SymmetricKey
	var ok bool
	if key, ok = mgr.keys[s]; !ok {
		var err error
		key, err = mgr.getKey(s)
		if err != nil {
			db.DPrintf(db.ERROR, "Error GetKey for signer %v: %v", s, err)
			return nil, fmt.Errorf("Error GetKey for signer %v: %v", s, err)
		}
		mgr.keys[s] = key
	} else {
		db.DPrintf(db.AUTH, "Key for signer %v cached", s)
	}
	return key, nil
}

func (mgr *KeyMgr) AddKey(s sp.Tsigner, key SymmetricKey) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.keys[s] = key
	db.DPrintf(db.AUTH, "Add key for signer %v", s)
}

func NewSymmetricKey(nbyte int) (SymmetricKey, error) {
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
	return SymmetricKey(key), nil
}

func (sk SymmetricKey) String() string {
	return string(sk)
}

func WithConstGetKeyFn(key SymmetricKey) GetKeyFn {
	return func(sp.Tsigner) (SymmetricKey, error) {
		return key, nil
	}
}
