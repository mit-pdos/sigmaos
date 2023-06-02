package main

import (
	"os"
	db "sigmaos/debug"
	"sigmaos/maze"
	"strconv"
)

func main() {
	// TODO add call arguments (i.e. width, height)
	// For now, all arguments are their defaults
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v public", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		db.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := maze.RunMaze(public); err != nil {
		db.DFatalf("RunMaze %v err %v\n", os.Args[0], err)
	}
}
