package nps3

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

type Nps3 struct {
	mu     sync.Mutex
	pid    string
	fssrv  *fssrv.FsServer
	client *s3.Client
	nextId np.Tpath // XXX delete?
	ch     chan bool
	root   *Dir
	*proc.ProcCtl
}

func MakeNps3(pid string) *Nps3 {
	nps3 := &Nps3{}
	nps3.pid = pid
	nps3.ch = make(chan bool)
	db.Name("nps3d")
	nps3.root = nps3.makeDir([]string{}, np.DMDIR, nil)

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("me-mit"))
	if err != nil {
		log.Fatalf("Failed to load SDK configuration %v", err)
	}

	nps3.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", kernel.S3, err)
	}
	nps3.fssrv = fssrv.MakeFsServer(nps3, nps3.root, ip+":0", fos.MakeProtServer(),
		false, "", nil)
	fsl := fslib.MakeFsLib("nps3")
	fsl.Mkdir(kernel.S3, 0777)
	err = fsl.PostServiceUnion(nps3.fssrv.MyAddr(), kernel.S3, nps3.fssrv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", nps3.fssrv.MyAddr(), err)
	}

	nps3dStartCond := usync.MakeCond(fsl, path.Join(kernel.BOOT, pid), nil)
	nps3dStartCond.Destroy()

	return nps3
}

func (nps3 *Nps3) Serve() {
	<-nps3.ch
}

func (nps3 *Nps3) Done() {
	nps3.ch <- true
}
