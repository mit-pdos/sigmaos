package main

import (
	"os"

	db "sigmaos/debug"
	fttasksrv "sigmaos/ft/task/srv"
)

func main() {
	if err := fttasksrv.RunTaskSrv(os.Args[1:]); err != nil {
		db.DFatalf("Start %v err %v\n", os.Args[0], err)
	}
}
