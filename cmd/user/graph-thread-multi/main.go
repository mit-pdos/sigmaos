package main

import (
	"os"
	db "sigmaos/debug"
	"sigmaos/graph"
	"strconv"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v public threadID jobnames... .", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	thread, err := strconv.Atoi(os.Args[2])
	if err != nil {
		db.DFatalf("Atoi %v err %v\n", os.Args[0], err)
	}
	if err := graph.StartBfsMultiThread(public, thread, os.Args[3:]); err != nil {
		db.DFatalf("StartBfsMultiThread %v err %v\n", os.Args[0], err)
	}
}
