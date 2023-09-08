package fss3

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/perf"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

var fss3 *Fss3

type Fss3 struct {
	*sigmasrv.SigmaSrv
	mu     sync.Mutex
	client *s3.Client
}

func RunFss3(buckets []string) {
	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}
	fss3 = &Fss3{}
	root := makeDir("", path.Path{}, sp.DMDIR)
	ssrv, err := sigmasrv.MakeSigmaSrvRoot(root, ip+":0", sp.S3, sp.S3REL)
	if err != nil {
		db.DFatalf("Error MakeSigmaSrv: %v", err)
	}
	p, err := perf.MakePerf(perf.S3)
	if err != nil {
		db.DFatalf("Error MakePerf: %v", err)
	}
	defer p.Done()

	fss3.SigmaSrv = ssrv
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("sigmaos"))
	if err != nil {
		db.DFatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	ssrv.RunServer()
}
