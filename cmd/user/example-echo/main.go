package main

import (
	"os"
	dbg "sigmaos/debug"
	echo "sigmaos/example_echo_server"
	"strconv"
)

func main() {
	if len(os.Args) != 2 {
		dbg.DFatalf("Usage: %v public", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		dbg.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := echo.RunEchoSrv(public); err != nil {
		dbg.DFatalf("RunEchoSrv %v err %v\n", os.Args[0], err)
	}
}
