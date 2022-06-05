package main

import (
	"fmt"
	"os"

	"ulambda/linuxsched"
	"ulambda/procd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v s3bin", os.Args[0])
		os.Exit(1)
	}
	if _, err := linuxsched.ScanTopology(); err != nil {
		fmt.Fprintf(os.Stderr, "ScanTopology failed %v\n", err)
		os.Exit(1)
	}
	procd.RunProcd(os.Args[1])
}
