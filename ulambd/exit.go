package ulambd

import (
	"log"
	"os"
)

func Exit(pid string) {
	ld := MakeLambd()
	err := ld.clnt.Rename(LDIR+"/"+pid+"/Running", LDIR+"/"+pid+"/Exit")
	if err != nil {
		log.Printf("Exit %v to %v error %v\n",
			pid+"/Running", pid+"/Exit", err)
	}
	os.Exit(0)
}
