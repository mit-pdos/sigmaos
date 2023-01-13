package main

import (
	//"bytes"
	//"fmt"
	"log"
	"os"

	"sigmaos/boot"
	//"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	//"sigmaos/frame"
	"sigmaos/kernel"
	"sigmaos/proc"
	"sigmaos/yaml"
)

const yml = "/home/sigmaos/bootkernelclnt/boot.yml"

var envvar = []string{proc.SIGMADEBUG, proc.SIGMAPERF, proc.SIGMANAMED, proc.SIGMAROOTFS, proc.SIGMAREALM}

func main() {
	if len(os.Args) < 1 {
		db.DFatalf("%v: usage\n", os.Args[0])
	}

	param := kernel.Param{}
	err := yaml.ReadYaml(yml, &param)
	if err != nil {
		db.DFatalf("%v: ReadYaml %s\n", os.Args[0], yml)
	}

	os.Setenv(proc.SIGMADEBUG, "KERNEL;")
	os.Setenv(proc.SIGMANAMED, ":1111")
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":/home/sigmaos/bin/kernel:/home/sigmaos/bin/user")
	// os.Setenv(proc.SIGMAROOTFS, "")
	os.Setenv(proc.SIGMAREALM, "rootrealm")

	log.Printf("yaml %v env %v\n", param, os.Environ())

	_, err = boot.BootUp(&param)
	if err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}

	// db.DPrintf(db.KERNEL, "%v: shutting down %s\n", os.Args[0], s)
	//if err := sys.ShutDown(); err != nil {
	//	db.DFatalf("%v: Shutdown err %v\n", os.Args[0], err)
	//}
	os.Exit(0)
}

// func main() {
// 	if len(os.Args) < 1 {
// 		db.DFatalf("%v: usage\n", os.Args[0])
// 	}

// 	b, serr := frame.ReadFrame(os.Stdin)
// 	if serr != nil {
// 		db.DFatalf("%v: Scanf err %v\n", os.Args[0], serr)
// 	}

// 	param := kernel.Param{}
// 	err := yaml.ReadYamlRdr(bytes.NewReader(b), &param)
// 	if err != nil {
// 		db.DFatalf("%v: ReadYaml err %v\n", os.Args[0], err)
// 	}

// 	sys, err := boot.BootUp(&param)
// 	if err != nil {
// 		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
// 	}

// 	// let parent/bootclnt know that kernel has booted
// 	if _, err := fmt.Printf("%s %s\n", bootkernelclnt.RUNNING, sys.Ip()); err != nil {
// 		db.DFatalf("%v: Printf err %v\n", os.Args[0], err)
// 	}

// 	db.DPrintf(db.KERNEL, "%v: wait for shutdown\n", os.Args[0])

// 	s := ""
// 	_, err = fmt.Scanf("%s", &s)
// 	if err != nil {
// 		db.DFatalf("%v: Scanf err %v\n", os.Args[0], err)
// 	}
// 	if s != bootkernelclnt.SHUTDOWN {
// 		db.DFatalf("%v: oops wrong shutdown command %v\n", os.Args[0], s)
// 	}

// 	db.DPrintf(db.KERNEL, "%v: shutting down %s\n", os.Args[0], s)
// 	if err := sys.ShutDown(); err != nil {
// 		db.DFatalf("%v: Shutdown err %v\n", os.Args[0], err)
// 	}
// 	os.Exit(0)
// }
