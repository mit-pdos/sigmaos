package main

import (
	"os"
	dbg "sigmaos/debug"
	echo "sigmaos/example_echo_server"
)

func main() {
	if len(os.Args) != 1 {
		dbg.DFatalf("Usage: %v", os.Args[0])
		return
	}

	if err := echo.RunEchoSrv(); err != nil {
		dbg.DFatalf("RunEchoSrv %v err %v\n", os.Args[0], err)
	}
}
