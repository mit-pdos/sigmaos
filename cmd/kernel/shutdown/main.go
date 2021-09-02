package main

import (
	"ulambda/kernel"
)

func main() {
	s := kernel.MakeSystem(".")
	s.Shutdown()
}
