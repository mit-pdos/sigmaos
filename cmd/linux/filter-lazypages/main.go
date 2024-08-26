package main

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/lazypagessrv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <image-dir>\n", os.Args[0])
		os.Exit(1)
	}
	if err := lazypagessrv.FilterLazyPages(os.Args[1]); err != nil {
		db.DPrintf(db.ALWAYS, "FilterLazyPages err %w", err)
		os.Exit(1)
	}
}
