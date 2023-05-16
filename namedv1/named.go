package namedv1

import (
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"

	// "sigmaos/ctx"
	db "sigmaos/debug"
	// "sigmaos/fs"
	"sigmaos/memfssrv"
	// "sigmaos/perf"
	// "sigmaos/proc"
	// "sigmaos/repl"
	// "sigmaos/repldummy"
	// "sigmaos/replraft"
	sp "sigmaos/sigmap"
)

var (
	dialTimeout = 5 * time.Second

	endpoints = []string{"localhost:2379", "localhost:22379", "localhost:32379"}
)

type Named struct {
	*memfssrv.MemFs
	mu   sync.Mutex
	clnt *clientv3.Client
}

func Run(args []string) {
	nd := &Named{}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		db.DFatalf("Error New %v\n", err)
	}
	nd.clnt = cli

	mfs, err := memfssrv.MakeMemFs(sp.NAMEDV1, sp.NAMEDV1REL)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	nd.MemFs = mfs

	mfs.Serve()
	mfs.Done()

	cli.Close() // make sure to close the client
}
