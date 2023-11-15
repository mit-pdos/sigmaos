package fss3

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/perf"
	pcfg "sigmaos/proc"
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
	fss3 = &Fss3{}
	ssrv, err := sigmasrv.NewSigmaSrvNoRPC(sp.S3, pcfg.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.S3)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	commonBuckets := []string{"9ps3", "sigma-common"}
	buckets = append(buckets, commonBuckets...)
	for _, bucket := range buckets {
		d := newDir(bucket, path.Path{}, sp.DMDIR)
		if err := ssrv.MkNod(bucket, d); err != nil {
			db.DFatalf("Error MkNod bucket in RunFss3: %v", err)
		}
	}
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
