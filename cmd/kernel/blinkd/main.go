package main

import (
	"os"

	blinksrv "sigmaos/blink/srv"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v kernelId\nPassed: %v", os.Args[0], os.Args)
	}
	kernelId := os.Args[1]
	if err := blinksrv.RunBlinkSrv(kernelId); err != nil {
		db.DFatalf("Fatal start: %v %v\n", os.Args[0], err)
	}
}
