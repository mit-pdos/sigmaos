package main

import (
	"fmt"
	"os"

	"ulambda/procd"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v realmbin coreIv", os.Args[0])
		os.Exit(1)
	}
	procd.RunProcd(os.Args[1], os.Args[2])
}
