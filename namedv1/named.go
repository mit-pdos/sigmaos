package namedv1

import (
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"

	// "sigmaos/ctx"
	"sigmaos/container"
	db "sigmaos/debug"
	// "sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/path"
	"sigmaos/sesssrv"
	sp "sigmaos/sigmap"
)

var (
	dialTimeout = 5 * time.Second

	endpoints = []string{"localhost:2379", "localhost:22379", "localhost:32379"}
)

var nd *Named

type Named struct {
	*sesssrv.SessSrv
	mu   sync.Mutex
	clnt *clientv3.Client
}

func Run(args []string) {
	nd = &Named{}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		db.DFatalf("Error New %v\n", err)
	}
	nd.clnt = cli

	root := makeDir(makeObj(path.Path{}, sp.DMDIR, 0, nil))

	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("LocalIP %v %v\n", sp.UX, err)
	}

	srv, err := fslibsrv.MakeReplServer(root, ip+":0", sp.NAMEDV1, "namedv1", nil)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	nd.SessSrv = srv

	srv.Serve()
	srv.Done()

	cli.Close() // make sure to close the client
}
