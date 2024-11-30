package main

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/lazypages/srv"
)

func main() {
	if len(os.Args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage:\n", os.Args[0])
		os.Exit(1)
	}
	if err := srv.Run(); err != nil {
		db.DFatalf("lazypagessrv: err %w", err)
	}
}
