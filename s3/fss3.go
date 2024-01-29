package fss3

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/perf"
	proc "sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

var fss3 *Fss3

type Fss3 struct {
	*sigmasrv.SigmaSrv
	mu sync.Mutex
	// TODO: don't create a new client for each PID... create a new client for
	// each principal...
	//	clients map[string]*s3.Client
	client *s3.Client
}

func RunFss3(buckets []string) {
	fss3 = &Fss3{}
	root := newDir("", path.Path{}, sp.DMDIR)
	pe := proc.GetProcEnv()
	addr := sp.NewTaddrAnyPort(sp.INNER_CONTAINER_IP, pe.GetNet())
	ssrv, err := sigmasrv.NewSigmaSrvRoot(root, sp.S3, addr, pe)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.S3)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	fss3.SigmaSrv = ssrv
	secrets, err := auth.GetAWSSecrets()
	if err != nil {
		db.DFatalf("Failed to load AWS secrets %v", err)
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(auth.NewAWSCredentialsProvider(secrets)))

	if err != nil {
		db.DFatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	ssrv.RunServer()
}
