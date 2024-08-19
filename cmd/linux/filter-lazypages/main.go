package main

import (
	"fmt"
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/lazypagessrv"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <image-dir> <pid>\n", os.Args[0])
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Atoi err %v\n", err)
		os.Exit(1)
	}

	if err := lazypagessrv.FilterLazyPages(os.Args[1], pid); err != nil {
		db.DPrintf(db.ALWAYS, "FilterLazyPages err %w", err)
		os.Exit(1)
	}
}
