package main

import (
	"log"
	"path"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"sigmaos/fslib"
	"sigmaos/semclnt"
)

func spin(args []string) error {
	addr := args[0]
	sempath := args[1]
	fsl, err := fslib.MakeFsLibNamed("spin-"+path.Base(sempath), []string{addr})
	if err != nil {
		return err
	}
	sem := semclnt.MakeSemClnt(fsl, sempath)
	err = sem.Up()
	if err != nil {
		return err
	}
	log.Printf("Addr %v sem %v", addr, path.Base(sempath))
	time.Sleep(14 * time.Second)
	log.Printf("Done sleep")
	return nil
}

func main() {
	lambda.Start(spin)
}
