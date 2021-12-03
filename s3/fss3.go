package fss3

import (
	"context"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/procclnt"
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

func RunFss3() {
	fss3 := &Fss3{}
	root := fss3.makeDir([]string{}, np.DMDIR, nil)
	fsl := fslib.MakeFsLib("fss3d")
	pclnt := procclnt.MakeProcClnt(fsl)
	srv, err := fslibsrv.MakeSrv(root, named.S3, fsl, pclnt)
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

	srv.Serve()
}
