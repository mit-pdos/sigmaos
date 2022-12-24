package fss3

import (
	"context"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/path"
	"sigmaos/perf"
	sp "sigmaos/sigmap"
)

var fss3 *Fss3

type Fss3 struct {
	*memfssrv.MemFs
	mu     sync.Mutex
	client *s3.Client
}

func RunFss3(buckets []string) {
	fss3 = &Fss3{}
	mfs, _, _, err := memfssrv.MakeMemFs(sp.S3, sp.S3REL)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	p := perf.MakePerf("S3")
	defer p.Done()

	commonBuckets := []string{"9ps3", "sigma-common"}
	buckets = append(buckets, commonBuckets...)
	for _, bucket := range buckets {
		// Add the 9ps3 bucket.
		d := makeDir(bucket, path.Path{}, sp.DMDIR)
		log.Printf("bucket %v d %v\n", bucket, d)
		if err := mfs.MkNod(bucket, d); err != nil {
			db.DFatalf("Error MkNod bucket in RunFss3: %v", err)
		}
	}
	fss3.MemFs = mfs
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		db.DFatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	mfs.Serve()
	mfs.Done()
}
