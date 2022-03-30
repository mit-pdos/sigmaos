package fss3

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/sesssrv"
)

var bucket = "9ps3"

const (
	CHUNKSZ = 8192
)

type Fss3 struct {
	*sesssrv.SessSrv
	mu     sync.Mutex
	client *s3.Client
}

func RunFss3() {
	fss3 := &Fss3{}
	root := fss3.makeDir(np.Path{}, np.DMDIR, nil)
	fsl := fslib.MakeFsLib("fss3d")
	pclnt := procclnt.MakeProcClnt(fsl)
	srv, err := fslibsrv.MakeSrv(root, np.S3, fsl, pclnt)
	if err != nil {
		db.DFatalf("%v: MakeSrv %v\n", proc.GetProgram(), err)
	}

	fss3.SessSrv = srv
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		db.DFatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	srv.Serve()
	srv.Done()
}
