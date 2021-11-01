package fss3

import (
	"context"
	"log"
	"path"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/named"
	np "ulambda/ninep"
	usync "ulambda/sync"
)

var bucket = "9ps3"

const (
	CHUNKSZ = 8192
)

type Fss3 struct {
	*fssrv.FsServer
	mu     sync.Mutex
	client *s3.Client
}

func RunFss3(pid string) {
	fss3 := &Fss3{}
	root := fss3.makeDir([]string{}, np.DMDIR, nil)
	srv, fsl, err := fslibsrv.MakeSrvClt(root, named.S3, "fss3d")
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}

	fss3.FsServer = srv
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		log.Fatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	fss3dStartCond := usync.MakeCond(fsl, path.Join(named.BOOT, pid), nil, true)
	fss3dStartCond.Destroy()

	fss3.Serve()
}
