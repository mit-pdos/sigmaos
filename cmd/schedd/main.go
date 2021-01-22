package main

import (
	"ulambda/schedd"
)

func main() {
	ld := schedd.MakeSchedd(false)
	ld.Scheduler()
}
