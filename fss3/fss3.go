package fss3

import (
	"context"
	"log"
	"path"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	fos "ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/kernel"
	np "ulambda/ninep"
	"ulambda/proc"
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
	root   *Dir
	*proc.ProcCtl
}

func MakeFss3(pid string) *Fss3 {
	fss3 := &Fss3{}
	fss3.pid = pid
	fss3.ch = make(chan bool)
	db.Name("fss3d")
	fss3.root = fss3.makeDir([]string{}, np.DMDIR, nil)

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		log.Fatalf("Failed to load SDK configuration %v", err)
	}

	fss3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", kernel.S3, err)
	}
	fss3.fssrv = fssrv.MakeFsServer(fss3, fss3.root, ip+":0", fos.MakeProtServer(),
		false, "", nil)
	fsl := fslib.MakeFsLib("fss3")
	fsl.Mkdir(kernel.S3, 0777)
	err = fsl.PostServiceUnion(fss3.fssrv.MyAddr(), kernel.S3, fss3.fssrv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", fss3.fssrv.MyAddr(), err)
	}

	fss3dStartCond := usync.MakeCond(fsl, path.Join(kernel.BOOT, pid), nil)
	fss3dStartCond.Destroy()

	return fss3
}

func (fss3 *Fss3) Serve() {
	<-fss3.ch
}

func (fss3 *Fss3) Done() {
	fss3.ch <- true
}
