package main

import (
	//	"log"
	//	"path"
	//	"time"

	"github.com/aws/aws-lambda-go/lambda"
	//	"sigmaos/sigmaclnt/fslib"
	//	"sigmaos/util/coordination/barrier"
	//	sp "sigmaos/sigmap"
	db "sigmaos/debug"
)

func spin(args []string) error {
	db.DFatalf("Error: env")
	// addr := args[0]
	// sempath := args[1]
	// fsl, err := fslib.NewFsLibAddr(sp.Tuname("spin-"+path.Base(sempath)), sp.ROOTREALM, "XXXXXXX", sp.NewTaddrs([]string{addr}))
	//
	//	if err != nil {
	//		return err
	//	}
	//
	// sem := barrier.NewBarrier(fsl, sempath)
	// err = sem.Up()
	//
	//	if err != nil {
	//		return err
	//	}
	//
	// log.Printf("Addr %v sem %v", addr, path.Base(sempath))
	// time.Sleep(14 * time.Second)
	// log.Printf("Done sleep")
	// return nil
	return nil
}

func main() {
	lambda.Start(spin)
}
