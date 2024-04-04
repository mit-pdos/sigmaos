package keysrv

import (
	"path"
	"sync"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/keys"
	"sigmaos/keysrv/proto"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type rOnlyKeySrv struct {
	mu   sync.Mutex
	keys map[sp.Tsigner][]byte
}

type rwKeySrv struct {
	*rOnlyKeySrv
}

type KeySrv struct {
	*rwKeySrv
}

func newrOnlyKeySrv(masterPubKey auth.PublicKey) *rOnlyKeySrv {
	return &rOnlyKeySrv{
		keys: map[sp.Tsigner][]byte{
			auth.SIGMA_DEPLOYMENT_MASTER_SIGNER: masterPubKey.B64(),
		},
	}
}

func newrwKeySrv(masterPubKey auth.PublicKey) *rwKeySrv {
	return &rwKeySrv{
		rOnlyKeySrv: newrOnlyKeySrv(masterPubKey),
	}
}

func NewKeySrv(masterPubKey auth.PublicKey) *KeySrv {
	return &KeySrv{
		rwKeySrv: newrwKeySrv(masterPubKey),
	}
}

func (ks rOnlyKeySrv) GetKey(ctx fs.CtxI, req proto.GetKeyRequest, res *proto.GetKeyResponse) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	s := sp.Tsigner(req.SignerStr)
	if b, ok := ks.keys[s]; ok {
		res.OK = true
		res.B64 = b
		db.DPrintf(db.KEYD, "Got key for signer %v", s)
	} else {
		db.DPrintf(db.KEYD, "No key for signer %v", s)
	}
	return nil
}

func (ks rwKeySrv) SetKey(ctx fs.CtxI, req proto.SetKeyRequest, res *proto.SetKeyResponse) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	s := sp.Tsigner(req.SignerStr)
	ks.keys[s] = req.GetB64()
	db.DPrintf(db.KEYD, "Set key for signer %v", s)
	return nil
}

func RunKeySrv(masterPubKey auth.PublicKey, masterPrivKey auth.PrivateKey) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	ks := NewKeySrv(masterPubKey)
	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithLocalMapGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, &ks.mu, ks.keys),
		masterPubKey,
		masterPrivKey,
		sp.Tsigner(sc.ProcEnv().GetKernelID()),
		masterPubKey,
		nil,
	)
	// Add the master deployment key, to allow connections from kernel to this
	// named.
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf("Error New authsrv: %v", err)
	}
	sc.SetAuthSrv(as)
	ssrv, err := sigmasrv.NewSigmaSrvClntKeyMgr(sp.KEYD, sc, kmgr, ks)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	// Add a directory for the RW rpc device
	if _, err := ssrv.Create(sp.RW_REL, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		db.DFatalf("Error Create RW rpc dev dir: %v", err)
	}
	if _, err := ssrv.Create(sp.RONLY_REL, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		db.DFatalf("Error Create RONLY rpc dev dir: %v", err)
	}
	if err := ssrv.AddRPCSrv(path.Join(sp.RW_REL, rpc.RPC), ks.rwKeySrv); err != nil {
		db.DFatalf("Error add RW rpc dev: %v", err)
	}
	if err := ssrv.AddRPCSrv(path.Join(sp.RONLY_REL, rpc.RPC), ks.rOnlyKeySrv); err != nil {
		db.DFatalf("Error add RONLY rpc dev: %v", err)
	}
	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.KEYD)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()
	db.DPrintf(db.KEYD, "Keyd running!")
	ssrv.RunServer()
}
