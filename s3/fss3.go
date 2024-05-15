package fss3

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/keys"
	"sigmaos/path"
	"sigmaos/perf"
	proc "sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

var fss3 *Fss3

type Fss3 struct {
	*sigmasrv.SigmaSrv
	mu      sync.Mutex
	clients map[sp.TprincipalID]*s3.Client
}

func (fss3 *Fss3) getClient(ctx fs.CtxI) (*s3.Client, *serr.Err) {
	fss3.mu.Lock()
	defer fss3.mu.Unlock()

	var clnt *s3.Client
	var ok bool
	if clnt, ok = fss3.clients[ctx.Principal().GetID()]; ok {
		return clnt, nil
	}
	s3secrets, ok := ctx.Claims().GetSecrets()["s3"]
	// If this principal doesn't carry any s3 secrets, return EPERM
	if !ok {
		return nil, serr.NewErr(serr.TErrPerm, fmt.Errorf("Principal %v has no S3 secrets", ctx.Principal().GetID()))
	}
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(
			auth.NewAWSCredentialsProvider(s3secrets),
		),
		config.WithRegion(`us-east-1`),
	)
	if err != nil {
		db.DFatalf("Failed to load SDK configuration %v", err)
	}
	clnt = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	fss3.clients[ctx.Principal().GetID()] = clnt
	return clnt, nil
}

func RunFss3(masterPubKey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc),
		masterPubKey,
		nil,
		sp.Tsigner(pe.GetPID()),
		pubkey,
		privkey,
	)
	amgr, err := auth.NewAuthMgr[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf("Error NewAuthMgr %v", err)
	}
	sc.SetAuthMgr(amgr)
	fss3 = &Fss3{
		clients: make(map[sp.TprincipalID]*s3.Client),
	}
	root := newDir("", path.Path{}, sp.DMDIR)
	addr := sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP, pe.GetNet())
	ssrv, err := sigmasrv.NewSigmaSrvRootClnt(root, sp.S3, addr, sc)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.S3)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	fss3.SigmaSrv = ssrv
	ssrv.RunServer()
}
