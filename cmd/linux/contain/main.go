package main

import (
	"log"
	"os"
	"os/exec"
	"sigmaos/container"
)

var defaultEnvironment = []string{
	"HOME=/root",
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"TERM=xterm",
	"NAMED=10.100.42.124:1111",
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("%s: Usage <bin> [args]\n", os.Args[0])
	}
	cmd := &exec.Cmd{
		Path: os.Args[1],
		Args: os.Args[1:],
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = defaultEnvironment
	if err := container.RunContainer(cmd); err != nil {
		log.Fatalf("%s: run container err %v\n", os.Args[0], err)
	}
	log.Printf("container done\n")
	if err := cmd.Wait(); err != nil {
		log.Fatalf("%s: wait err %v\n", os.Args[0], err)
	}
}
