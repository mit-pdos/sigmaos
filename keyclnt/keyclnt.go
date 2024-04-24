package keyclnt

import (
	"fmt"
	"sync"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/keysrv/proto"
	"sigmaos/rpcclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
)

type KeyClnt[M jwt.SigningMethod] struct {
	mu    sync.Mutex
	sc    *sigmaclnt.SigmaClnt
	ronly *rpcclnt.RPCClnt
	rw    *rpcclnt.RPCClnt
}

func NewKeyClnt[M jwt.SigningMethod](sc *sigmaclnt.SigmaClnt) *KeyClnt[M] {
	return &KeyClnt[M]{
		sc: sc,
	}
}

func (kc *KeyClnt[M]) getClnt(rw bool) (*rpcclnt.RPCClnt, error) {
	db.DPrintf(db.KEYCLNT, "getClnt rw %v", rw)
	defer db.DPrintf(db.KEYCLNT, "getClnt done rw %v", rw)

	kc.mu.Lock()
	defer kc.mu.Unlock()

	if rw {
		if kc.rw == nil {
			mnt, err := kc.sc.ReadEndpoint(sp.KEYD)
			if err != nil {
				db.DPrintf(db.ERROR, "Error ReadEndpoint: %v", err)
				return nil, err
			}
			err = kc.sc.MountTree(mnt, sp.RW_REL, sp.KEYS_RW)
			if err != nil {
				db.DPrintf(db.KEYCLNT_ERR, "Error MountTree: %v", err)
				return nil, err
			}
			ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{kc.sc.FsLib}, sp.KEYS_RW)
			if err != nil {
				db.DPrintf(db.ERROR, "Error new RPC clnt rw: %v", err)
				return nil, err
			}
			rpcc := rpcclnt.NewRPCClnt(ch)
			kc.rw = rpcc
		}
		return kc.rw, nil
	} else {
		if kc.ronly == nil {
			mnt, err := kc.sc.ReadEndpoint(sp.KEYD)
			if err != nil {
				db.DPrintf(db.ERROR, "Error ReadEndpoint: %v", err)
				return nil, err
			}
			err = kc.sc.MountTree(mnt, sp.RONLY_REL, sp.KEYS_RONLY)
			if err != nil {
				db.DPrintf(db.ERROR, "Error MountTree: %v", err)
				return nil, err
			}
			ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{kc.sc.FsLib}, sp.KEYS_RONLY)
			if err != nil {
				db.DPrintf(db.ERROR, "Error new RPC clnt ronly: %v", err)
				return nil, err
			}
			rpcc := rpcclnt.NewRPCClnt(ch)
			kc.ronly = rpcc
		}
		return kc.ronly, nil
	}
}

func (kc *KeyClnt[M]) GetKey(signingMethod M, s sp.Tsigner) (auth.PublicKey, error) {
	db.DPrintf(db.KEYCLNT, "GetKey for signer %v", s)
	defer db.DPrintf(db.KEYCLNT, "GetKey done for signer %v", s)

	rpcc, err := kc.getClnt(false)
	if err != nil {
		return nil, err
	}
	req := &proto.GetKeyRequest{
		SignerStr: string(s),
	}
	res := &proto.GetKeyResponse{}
	db.DPrintf(db.KEYCLNT, "Try to GetKey for signer %v", s)
	if err := rpcc.RPC("rOnlyKeySrv.GetKey", req, res); err != nil {
		db.DPrintf(db.ERROR, "rOnlyKeySrv.GetKey err %v", err)
		return nil, err
	}
	db.DPrintf(db.KEYCLNT, "GetKey for signer %v res %v", s, res.GetOK())
	if !res.GetOK() {
		return nil, fmt.Errorf("Error: no key for signer %v", s)
	}
	pubkey, err := auth.NewPublicKey[M](signingMethod, res.GetB64())
	if err != nil {
		db.DPrintf(db.ERROR, "NewPublicKey err %v", err)
		return nil, err
	}
	return pubkey, nil
}

func (kc *KeyClnt[M]) SetKey(s sp.Tsigner, key auth.PublicKey) error {
	db.DPrintf(db.KEYCLNT, "SetKey for signer %v", s)
	defer db.DPrintf(db.KEYCLNT, "SetKey done for signer %v", s)

	rpcc, err := kc.getClnt(true)
	if err != nil {
		return err
	}
	req := &proto.SetKeyRequest{
		SignerStr: string(s),
		B64:       key.B64(),
	}
	res := &proto.SetKeyResponse{}
	db.DPrintf(db.KEYCLNT, "Try to SetKey for signer %v", s)
	if err := rpcc.RPC("rwKeySrv.SetKey", req, res); err != nil {
		db.DPrintf(db.ERROR, "rwKeySrv.SetKey err %v", err)
		return err
	}
	db.DPrintf(db.KEYCLNT, "SetKey done for signer %v", s)
	return nil
}
