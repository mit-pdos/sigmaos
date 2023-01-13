package main

import (
	"log"
	"os"
	"path"

	db "sigmaos/debug"
	"sigmaos/ux"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v rootux", os.Args[0])
	}
	rootux := os.Args[1]
	log.Printf("root ux %v\n", rootux)
	os.MkdirAll(path.Join(rootux, "bin", "user"), 0755)
	fsux.RunFsUx(rootux)
}
