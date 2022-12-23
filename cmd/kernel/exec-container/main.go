package main

import (
	"os"
	"sigmaos/container"
)

func main() {
	container.ExecContainer()
	os.Exit(0)
}
