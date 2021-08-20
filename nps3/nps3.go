package nps3

import (
	"context"
	"log"
	"net"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
	"ulambda/stats"
)

var bucket = "9ps3"

const (
	CHUNKSZ = 8192
)

type Nps3 struct {
	mu     sync.Mutex
	srv    *npsrv.NpServer
	client *s3.Client
	nextId np.Tpath // XXX delete?
	ch     chan bool
	st     *npo.SessionTable
	root   *Dir
}

func MakeNps3() *Nps3 {
	nps3 := &Nps3{}
	nps3.ch = make(chan bool)
	db.Name("nps3d")
	nps3.root = nps3.makeDir([]string{}, np.DMDIR, nil)
	nps3.st = npo.MakeSessionTable()

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
	nps3.srv = npsrv.MakeNpServer(nps3, ip+":0")
	fsl := fslib.MakeFsLib("nps3")
	fsl.Mkdir(kernel.S3, 0777)
	err = fsl.PostServiceUnion(nps3.srv.MyAddr(), kernel.S3, nps3.srv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", nps3.srv.MyAddr(), err)
	}

	return nps3
}

func (nps3 *Nps3) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(nps3, nps3.srv.GetFsServer(), conn)
}

func (nps3 *Nps3) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return nps3.root, nil
}

func (nps3 *Nps3) Serve() {
	<-nps3.ch
}

func (nps3 *Nps3) Done() {
	nps3.ch <- true
}

func (nps3 *Nps3) WatchTable() *npo.WatchTable {
	return nil
}

func (nps3 *Nps3) ConnTable() *npo.ConnTable {
	return nil
}

func (nps3 *Nps3) SessionTable() *npo.SessionTable {
	return nps3.st
}

func (nps3 *Nps3) RegisterSession(sess np.Tsession) {
	nps3.st.RegisterSession(sess)
}

func (nps3 *Nps3) Stats() *stats.Stats {
	return nil
}
