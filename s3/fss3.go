package fss3

import (
	"context"
	"log"
	"path"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"ulambda/fs"
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
	mu     sync.Mutex
	pid    string
	fssrv  *fssrv.FsServer
	client *s3.Client
	nextId np.Tpath // XXX delete?
	ch     chan bool
	root   fs.Dir
}

func MakeFss3(pid string) *Fss3 {
	fss3 := &Fss3{}
	fss3.pid = pid
	fss3.ch = make(chan bool)
	fss3.root = fss3.makeDir([]string{}, np.DMDIR, nil)
	srv, fsl, err := fslibsrv.MakeSrvFsLib(fss3, fss3.root, named.S3, "fss3d")
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}

	fss3.fssrv = srv
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		log.Fatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	fss3dStartCond := usync.MakeCond(fsl, path.Join(named.BOOT, pid), nil)
	fss3dStartCond.Destroy()

	return fss3
}

func (fss3 *Fss3) Serve() {
	<-fss3.ch
}

func (fss3 *Fss3) Done() {
	fss3.ch <- true
}
