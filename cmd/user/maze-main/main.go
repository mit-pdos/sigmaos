package main

import (
	"os"
	db "sigmaos/debug"
	maze "sigmaos/mazes"
)

func main() {
	// TODO add call arguments (i.e. width, height)
	// For now, all arguments are their defaults
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
		return
	}
	if err := maze.RunMaze(); err != nil {
		db.DFatalf("RunMaze %v err %v\n", os.Args[0], err)
	}
}
