package main

import (
	"log"
	"os"
	"os/exec"

	"sigmaos/container"
)

var defaultEnvironment = []string{
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"TERM=xterm",
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("%s: Usage <bin> <realm> [args]\n", os.Args[0])
	}
	cmd := &exec.Cmd{
		Path: os.Args[2],
		Args: os.Args[2:],
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	env := container.MakeEnv()
	for _, s := range defaultEnvironment {
		env = append(env, s)
	}
	cmd.Env = env
	if err := container.RunKernelContainer(cmd, os.Args[1]); err != nil {
		log.Fatalf("%s: run container err %v\n", os.Args[0], err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatalf("%s: wait err %v\n", os.Args[0], err)
	}
	if err := container.DelScnet(cmd.Process.Pid, os.Args[1]); err != nil {
		log.Fatalf("%s: failed to delete bridge %v\n", os.Args[0], err)
	}
}
