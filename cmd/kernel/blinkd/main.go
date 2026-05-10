package main

import (
	"flag"
	"os"

	blinksrv "sigmaos/blink/srv"
	db "sigmaos/debug"
)

var nfsAddr = flag.String("nfs", "", "NFS server IP address to pass to the chroot mount script")

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		db.DFatalf("Usage: %v [--nfs IP] kernelId\nPassed: %v", os.Args[0], os.Args)
	}
	kernelId := flag.Arg(0)
	if err := blinksrv.RunBlinkSrv(kernelId, *nfsAddr); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
