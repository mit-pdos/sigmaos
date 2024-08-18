package main

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/lazypagessrv"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <image-dir> <pages-dir>\n", os.Args[0])
		os.Exit(1)
	}
	lps := lazypagessrv.NewLazyPageSrv(os.Args[1], os.Args[2])
	if err := lps.Run(); err != nil {
		db.DPrintf(db.ALWAYS, "lazypagessrv: err %w", err)
		os.Exit(1)
	}
}
