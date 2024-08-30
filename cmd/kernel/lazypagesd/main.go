package main

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/lazypagessrv"
)

func main() {
	if len(os.Args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage:\n", os.Args[0])
		os.Exit(1)
	}
	if err := lazypagessrv.Run(); err != nil {
		db.DPrintf(db.ALWAYS, "lazypagessrv: err %w", err)
		os.Exit(1)
	}
}
