package main

import (
	"fmt"
	"os"
	"sigmaos/boot"

	"sigmaos/bootclnt"
	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 3 {
		db.DFatalf("%v: usage realmid param.yml\n", os.Args[0])
	}
	sys, err := kernel.BootUp(os.Args[1], os.Args[2])
	if err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}

	// let parent/bootclnt know that kernel has booted
	if _, err := fmt.Printf("%s\n", bootclnt.RUNNING); err != nil {
		db.DFatalf("%v: Printf err %v\n", os.Args[0], err)
	}

	db.DPrintf(db.KERNEL, "%v: wait for shutdown\n", os.Args[0])
	s := ""
	_, err = fmt.Scanf("%s", &s)
	if err != nil {
		db.DFatalf("%v: Scanf err %v\n", os.Args[0], err)
	}
	if s != bootclnt.SHUTDOWN {
		db.DFatalf("%v: oops wrong shutdown command %v\n", os.Args[0], s)
	}

	db.DPrintf(db.KERNEL, "%v: shutting down %s\n", os.Args[0], s)
	if err := sys.ShutDown(); err != nil {
		db.DFatalf("%v: Shutdown err %v\n", os.Args[0], err)
	}
	os.Exit(0)
}
