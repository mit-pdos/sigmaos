package ulamblib

import (
	"log"
	"os"

	"ulambda/fslib"
)

func Exit(pid string) {
	clnt := fslib.MakeFsLib(false)
	err := clnt.Rename(pid+"/Running", pid+"/Exit")
	if err != nil {
		log.Printf("Exit %v to %v error %v\n",
			pid+"/Running", pid+"/Exit", err)
	}
	err = clnt.WriteFile("name/ulambd/ulambd", []byte("hello"))
	if err != nil {
		log.Printf("Write ulambd dev failed %v\n", err)
	}
	os.Exit(0)
}
