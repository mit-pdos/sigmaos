package main

import (
	"os"
	"path"

	db "sigmaos/debug"
	"sigmaos/proxy/ux"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v rootux", os.Args[0])
	}
	rootux := os.Args[1]
	db.DPrintf(db.UX, "root ux %v\n", rootux)
	if err := os.MkdirAll(path.Join(rootux, "bin", "user"), 0755); err != nil {
		db.DFatalf("Error MkdirAll: %v", err)
	}
	fsux.RunFsUx(rootux)
}
