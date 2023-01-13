package main

import (
	"os"

	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/yaml"
)

func main() {
	if len(os.Args) < 1 {
		db.DFatalf("%v: usage\n", os.Args[0])
	}

	param := kernel.Param{}
	err := yaml.ReadYaml(os.Args[1], &param)
	if err != nil {
		db.DFatalf("%v: ReadYaml %s\n", os.Args[0], os.Args[1])
	}

	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":/home/sigmaos/bin/kernel:/home/sigmaos/bin/user")

	_, err = boot.BootUp(&param)
	if err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}

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
