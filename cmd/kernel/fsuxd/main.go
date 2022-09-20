package main

import (
	"os"
	"path"

	db "sigmaos/debug"
	"sigmaos/ux"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v root", os.Args[0])
	}
	root := os.Args[1]
	os.MkdirAll(path.Join(root, "bin", "user"), 0755)
	fsux.RunFsUx(root)
}
