package keys

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/keyclnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	MAX_RETRIES = 50
)

type GetKeyFn func(signer sp.Tsigner) (auth.PublicKey, error)

func WithConstGetKeyFn(key auth.PublicKey) GetKeyFn {
	return func(sp.Tsigner) (auth.PublicKey, error) {
		return key, nil
	}
}

func WithLocalMapGetKeyFn[M jwt.SigningMethod](signingMethod M, mu *sync.Mutex, m map[sp.Tsigner][]byte) GetKeyFn {
	return func(signer sp.Tsigner) (auth.PublicKey, error) {
		mu.Lock()
		defer mu.Unlock()

		if b, ok := m[signer]; ok {
			return auth.NewPublicKey[M](signingMethod, b)
		}
		return nil, fmt.Errorf("No key for signer %v in local map", signer)
	}
}

func WithSigmaClntGetKeyFn[M jwt.SigningMethod](signingMethod M, sc *sigmaclnt.SigmaClnt) GetKeyFn {
	kc := keyclnt.NewKeyClnt[M](sc)
	return func(signer sp.Tsigner) (auth.PublicKey, error) {
		db.DPrintf(db.KEYCLNT, "SigmaClntGetKey for signer %v", signer)
		defer db.DPrintf(db.KEYCLNT, "SigmaClntGetKey done for signer %v", signer)
		var key auth.PublicKey
		var err error
		// Mount the master key file, which should be mountable by anyone
		for i := 0; i < MAX_RETRIES; i++ {
			key, err = kc.GetKey(signingMethod, signer)
			if err == nil {
				break
			}
			if err != nil && !serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.ERROR, "Error get key: %v", err)
				return nil, err
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err != nil {
			db.DPrintf(db.ERROR, "Error get key: %v", err)
			return nil, err
		}
		return key, nil
	}
}

type KeyMgr struct {
	mu       sync.Mutex
	getKey   GetKeyFn
	pubkeys  map[sp.Tsigner]auth.PublicKey
	privkeys map[sp.Tsigner]auth.PrivateKey
}

func NewKeyMgr(fn GetKeyFn) *KeyMgr {
	return &KeyMgr{
		getKey:   fn,
		pubkeys:  make(map[sp.Tsigner]auth.PublicKey),
		privkeys: make(map[sp.Tsigner]auth.PrivateKey),
	}
}

func NewKeyMgrWithBootstrappedKeys(fn GetKeyFn, masterPubKey auth.PublicKey, masterPrivKey auth.PrivateKey, signer sp.Tsigner, pubkey auth.PublicKey, privkey auth.PrivateKey) *KeyMgr {
	kmgr := NewKeyMgr(fn)
	if masterPubKey != nil {
		kmgr.AddPublicKey(auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, masterPubKey)
	}
	if masterPrivKey != nil {
		kmgr.AddPrivateKey(auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, masterPrivKey)
	}
	if pubkey != nil {
		kmgr.AddPublicKey(signer, pubkey)
	}
	if privkey != nil {
		kmgr.AddPrivateKey(signer, privkey)
	}
	return kmgr
}

func (mgr *KeyMgr) GetPublicKey(s sp.Tsigner) (auth.PublicKey, error) {
	db.DPrintf(db.AUTH, "GetPublicKey for signer %v", s)
	defer db.DPrintf(db.AUTH, "GetPublicKey for signer %v done", s)
	mgr.mu.Lock()
	key, ok := mgr.pubkeys[s]
	mgr.mu.Unlock()
	if !ok {
		db.DPrintf(db.AUTH, "Key for signer %v not in map %v", s, mgr)
		var err error
		// Must not hold lock across getKey, which may be a circular RPC. getKey
		// should be deterministic, so not holding the lock (and setting the value
		// in the map twice) should be fine.
		key, err = mgr.getKey(s)
		if err != nil {
			db.DPrintf(db.ERROR, "Error GetPublicKey for signer %v: %v", s, err)
			return nil, fmt.Errorf("Error GetPublicKey for signer %v: %v", s, err)
		}
		mgr.mu.Lock()
		mgr.pubkeys[s] = key
		mgr.mu.Unlock()
	} else {
		db.DPrintf(db.AUTH, "GetPublicKey cached for signer %v", s)
	}
	return key, nil
}

func (mgr *KeyMgr) GetPrivateKey(s sp.Tsigner) (auth.PrivateKey, error) {
	db.DPrintf(db.AUTH, "GetPrivateKey for signer %v", s)
	defer db.DPrintf(db.AUTH, "GetPrivateKey for signer %v done", s)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if key, ok := mgr.privkeys[s]; !ok {
		return nil, fmt.Errorf("Error GetPrivateKey no key for signer %v: %v", s)
	} else {
		db.DPrintf(db.AUTH, "GetPrivateKey cached for signer %v", s)
		return key, nil
	}
}

func (mgr *KeyMgr) AddPublicKey(s sp.Tsigner, key auth.PublicKey) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.pubkeys[s] = key
	db.DPrintf(db.AUTH, "Add pub key for signer %v", s)
}

func (mgr *KeyMgr) AddPrivateKey(s sp.Tsigner, key auth.PrivateKey) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.privkeys[s] = key
	db.DPrintf(db.AUTH, "Add priv key for signer %v", s)
}

func (mgr *KeyMgr) String() string {
	pub := make([]sp.Tsigner, 0)
	priv := make([]sp.Tsigner, 0)
	for s, _ := range mgr.pubkeys {
		pub = append(pub, s)
	}
	for s, _ := range mgr.privkeys {
		priv = append(priv, s)
	}
	return fmt.Sprintf("%p=&{ pubkey-signers:%v pubkey-signers:%v }", mgr, pub, priv)
}
