package keys

import (
	"fmt"
	"sync"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func WithConstGetKeyFn(key auth.SymmetricKey) auth.GetKeyFn {
	return func(sp.Tsigner) (auth.SymmetricKey, error) {
		return key, nil
	}
}

func WithSigmaClntGetKeyFn(sc *sigmaclnt.SigmaClnt) auth.GetKeyFn {
	return func(signer sp.Tsigner) (auth.SymmetricKey, error) {
		// Mount the master key file, which should be mountable by anyone
		key, err := sc.GetFile(keyPath(signer))
		if err != nil {
			db.DPrintf(db.ERROR, "Error get master key: %v", err)
			return nil, err
		}
		return auth.SymmetricKey(key), nil
	}
}

type SymmetricKeyMgr struct {
	mu     sync.Mutex
	getKey auth.GetKeyFn
	keys   map[sp.Tsigner]auth.SymmetricKey
}

func NewSymmetricKeyMgr(fn auth.GetKeyFn) *SymmetricKeyMgr {
	return &SymmetricKeyMgr{
		getKey: fn,
		keys:   make(map[sp.Tsigner]auth.SymmetricKey),
	}
}

func (mgr *SymmetricKeyMgr) GetKey(s sp.Tsigner) (auth.SymmetricKey, error) {
	db.DPrintf(db.AUTH, "GetKey for signer %v", s)
	defer db.DPrintf(db.AUTH, "GetKey for signer %v done", s)
	mgr.mu.Lock()
	key, ok := mgr.keys[s]
	mgr.mu.Unlock()
	if !ok {
		db.DPrintf(db.AUTH, "Key for signer %v not in map %v", s, mgr)
		var err error
		// Must not hold lock across getKey, which may be a circular RPC. getKey
		// should be deterministic, so not holding the lock (and setting the value
		// in the map twice) should be fine.
		key, err = mgr.getKey(s)
		if err != nil {
			db.DPrintf(db.ERROR, "Error GetKey for signer %v: %v", s, err)
			return nil, fmt.Errorf("Error GetKey for signer %v: %v", s, err)
		}
		mgr.mu.Lock()
		mgr.keys[s] = key
		mgr.mu.Unlock()
	} else {
		db.DPrintf(db.AUTH, "GetKey cached for signer %v", s)
	}
	return key, nil
}

func (mgr *SymmetricKeyMgr) AddKey(s sp.Tsigner, key auth.SymmetricKey) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.keys[s] = key
	db.DPrintf(db.AUTH, "Add key for signer %v", s)
}

func (mgr *SymmetricKeyMgr) String() string {
	signers := make([]sp.Tsigner, 0)
	for s, _ := range mgr.keys {
		signers = append(signers, s)
	}
	return fmt.Sprintf("%p=&{ signers:%v }", mgr, signers)
}
