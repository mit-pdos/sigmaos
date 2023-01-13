package main

import (
	"net"
	"os"
	"os/exec"

	"sigmaos/container"
	db "sigmaos/debug"
)

var defaultEnvironment = []string{
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"TERM=xterm",
}

func main() {
	if len(os.Args) < 3 {
		db.DFatalf("%s: Usage <realm> <bin> [args]\n", os.Args[0])
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

	mybridge := true
	_, err := net.InterfaceByName(container.BridgeName(os.Args[1]))
	if err == nil {
		mybridge = false
	}

	if err := container.RunKernelContainer(cmd, os.Args[1]); err != nil {
		db.DFatalf("%s: run container err %v\n", os.Args[0], err)
	}
	if err := cmd.Wait(); err != nil {
		db.DFatalf("%s: wait err %v\n", os.Args[0], err)
	}
	if mybridge {
		if err := container.DelScnet(cmd.Process.Pid, os.Args[1]); err != nil {
			db.DFatalf("%s: failed to delete bridge %v\n", os.Args[0], err)
		}
	}
}
