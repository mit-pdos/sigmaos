package main

import (
	"fmt"
	"os"
	"sigmaos/kernel"

	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("%v: usage param.yml\n", os.Args[0])
	}
	sys, err := kernel.BootUp(os.Args[1])
	if err != nil {
		db.DFatalf("%v: boot %s err %v\n", os.Args[0], os.Args[1], err)
	}

	// let parent know what kernel has booted
	if _, err := fmt.Printf("running\n"); err != nil {
		db.DFatalf("%v: Printf err %v\n", os.Args[0], err)
	}

	db.DPrintf(db.KERNEL, "%v: wait for shutdown\n", os.Args[0])
	s := ""
	_, err = fmt.Scanf("%s", &s)
	if err != nil {
		db.DFatalf("%v: Scanf err %v\n", os.Args[0], err)
	}

	db.DPrintf(db.KERNEL, "%v: shutting down %s\n", os.Args[0], s)
	if err := sys.ShutDown(); err != nil {
		db.DFatalf("%v: Shutdown err %v\n", os.Args[0], err)
	}
	os.Exit(0)
}
