package main

import (
	"ulambda/schedd"
)

func main() {
	ld := schedd.MakeSchedd()
	ld.Scheduler()
}
