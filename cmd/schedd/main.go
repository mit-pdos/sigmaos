package main

import (
	"ulambda/lschedd"
)

func main() {
	ld := lschedd.MakeSchedd(false)
	ld.Scheduler()
}
