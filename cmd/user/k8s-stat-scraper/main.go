package main

import (
	"os"

	db "sigmaos/debug"
	k8sutil "sigmaos/util/k8s"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	err := k8sutil.RunK8sStatScraper()
	if err != nil {
		db.DFatalf("Err run k8s scraper: %v", err)
	}
}
