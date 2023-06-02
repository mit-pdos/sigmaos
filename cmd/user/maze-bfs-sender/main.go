package main

import (
	"os"
	db "sigmaos/debug"
	"sigmaos/maze"
	"strconv"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v public", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := maze.RunBFSSender(public); err != nil {
		db.DFatalf("RunBFSSender %v err %v\n", os.Args[0], err)
	}
}
