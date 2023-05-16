package namedv1

import (
	"context"
	"log"
	"time"

	"github.com/coreos/etcd/clientv3"
	// "sigmaos/ctx"
	// db "sigmaos/debug"
	// "sigmaos/fs"
	// "sigmaos/memfssrv"
	// "sigmaos/perf"
	// "sigmaos/proc"
	// "sigmaos/repl"
	// "sigmaos/repldummy"
	// "sigmaos/replraft"
	// sp "sigmaos/sigmap"
	// // "sigmaos/seccomp"
)

var (
	dialTimeout = 5 * time.Second

	endpoints = []string{"localhost:2379", "localhost:22379", "localhost:32379"}
)

func Run(args []string) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close() // make sure to close the client

	_, err = cli.Put(context.TODO(), "foo", "bar")
	if err != nil {
		log.Fatal(err)
	}
}
