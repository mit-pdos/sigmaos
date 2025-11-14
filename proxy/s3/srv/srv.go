package srv

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	proc "sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/auth"
	"sigmaos/util/perf"
)

var fss3 *Fss3

type Fss3 struct {
	*sigmasrv.SigmaSrv
	mu       sync.Mutex
	clients  map[sp.TprincipalID]*s3.Client
	S3RpcAPI *S3RpcAPI
}

func (fss3 *Fss3) getClient(ctx fs.CtxI) (*s3.Client, *serr.Err) {
	fss3.mu.Lock()
	defer fss3.mu.Unlock()

	var clnt *s3.Client
	var ok bool
	if clnt, ok = fss3.clients[ctx.Principal().GetID()]; ok {
		return clnt, nil
	}
	s3secrets, ok := ctx.Secrets()["s3"]
	// If this principal doesn't carry any s3 secrets, return EPERM
	if !ok {
		return nil, serr.NewErr(serr.TErrPerm, fmt.Errorf("Principal %v has no S3 secrets", ctx.Principal().GetID()))
	}
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(
			auth.NewAWSCredentialsProvider(s3secrets),
		),
		config.WithRegion(s3secrets.Metadata),
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

func RunFss3() {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	db.DPrintf(db.S3, "Start s3 %v", pe)
	fss3 = &Fss3{
		clients: make(map[sp.TprincipalID]*s3.Client),
	}
	fss3.S3RpcAPI = newRPCAPI(fss3)
	root := newDir("", path.Tpathname{}, sp.DMDIR)
	addr := sp.NewTaddrAnyPort()
	ssrv, err := sigmasrv.NewSigmaSrvRootClnt(root, addr, sp.ProxyPathname(sp.S3, pe.GetKernelID()), sc)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	if err := ssrv.MountRPCSrv(fss3.S3RpcAPI); err != nil {
		db.DFatalf("Err add S3RpcAPI: %v", err)
	}
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.S3)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	fss3.SigmaSrv = ssrv
	ssrv.RunServer()
}
