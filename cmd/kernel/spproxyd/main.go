package main

import (
	"flag"

	db "sigmaos/debug"
	"sigmaos/proxy/sigmap/srv"
)

func main() {
	blink := flag.Bool("blink", false, "also listen on TCP socket for blink")
	flag.Parse()
	if err := srv.RunSPProxySrv(*blink); err != nil {
		db.DFatalf("Fatal start: %v\n", err)
	}
}
