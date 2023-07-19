package fss3

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
)

var fss3 *Fss3

type Fss3 struct {
	*protdevsrv.ProtDevSrv
	mu     sync.Mutex
	client *s3.Client
}

func RunFss3(buckets []string) {
	fss3 = &Fss3{}
	psd, err := protdevsrv.MakeProtDevSrv(sp.S3, nil, sp.S3REL)
	if err != nil {
		db.DFatalf("Error MakeProtDevSrv: %v", err)
	}
	p, err := perf.MakePerf(perf.S3)
	if err != nil {
		db.DFatalf("Error MakePerf: %v", err)
	}
	defer p.Done()

	commonBuckets := []string{"9ps3", "sigma-common"}
	buckets = append(buckets, commonBuckets...)
	for _, bucket := range buckets {
		// Add the 9ps3 bucket.
		d := makeDir(bucket, path.Path{}, sp.DMDIR)
		if err := psd.MkNod(bucket, d); err != nil {
			db.DFatalf("Error MkNod bucket in RunFss3: %v", err)
		}
	}
	fss3.ProtDevSrv = psd
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		db.DFatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	psd.Serve()
	psd.Exit(proc.MakeStatus(proc.StatusEvicted))
}
