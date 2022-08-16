package main

import (
	"log"
	"path"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"ulambda/fslib"
	//		"ulambda/semclnt"
)

func spin(args []string) error {
	addr := args[0]
	sempath := args[1]
	log.Printf("Addr %v sem %v", addr, path.Base(sempath))
	fsl := fslib.MakeFsLibAddr("spin-"+path.Base(sempath), []string{addr})
	sts, err := fsl.GetDir("name/")
	if err != nil {
		return err
	}
	log.Printf("Got dir: %v", sts)
	//	sem := semclnt.MakeSemClnt(fsl, sempath)
	//	err := sem.Up()
	//	if err != nil {
	//		return err
	//	}
	time.Sleep(4 * time.Second)
	//	//	time.Sleep(4 * time.Minute)
	//	log.Printf("Done sleep sem %v", path.Base(sempath))
	return nil
}

func main() {
	lambda.Start(spin)
}
