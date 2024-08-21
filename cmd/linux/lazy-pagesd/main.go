package main

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/lazypagessrv"
)

// ./bin/linux/ckptsrv 100
// sudo criu dump -vvvv --images-dir dump --shell-job --log-file log.txt -t $(pgrep ckptclnt)
// cp -r dump dump1
//   ./bin/linux/lazy-pages dump1 dump/pages-1.img
// or
//   sudo criu lazy-pages -D dump1
// sudo criu restore -D dump1 --shell-job --lazy-pages

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <image-dir> <pages>\n", os.Args[0])
		os.Exit(1)
	}
	lps, err := lazypagessrv.NewLazyPagesSrv(os.Args[1], os.Args[2])
	if err != nil {
		db.DPrintf(db.ALWAYS, "NewLazyPagesSrv: err %w", err)
		os.Exit(1)
	}
	if err := lps.Run(); err != nil {
		db.DPrintf(db.ALWAYS, "lazypagessrv: err %w", err)
		os.Exit(1)
	}
}
