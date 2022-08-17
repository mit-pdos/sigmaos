package main

import (
	"log"
	"path"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"ulambda/fslib"
	"ulambda/semclnt"
)

func spin(args []string) error {
	addr := args[0]
	sempath := args[1]
	fsl := fslib.MakeFsLibAddr("spin-"+path.Base(sempath), []string{addr})
	sem := semclnt.MakeSemClnt(fsl, sempath)
	err := sem.Up()
	if err != nil {
		return err
	}
	log.Printf("Addr %v sem %v", addr, path.Base(sempath))
	time.Sleep(2 * time.Minute)
	log.Printf("Done sleep")
	return nil
}

func main() {
	lambda.Start(spin)
}
