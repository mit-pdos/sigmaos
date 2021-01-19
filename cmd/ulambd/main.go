package main

import (
	"ulambda/ulambdad"
)

func main() {
	ld := ulambd.MakeLambd(false)
	ld.Scheduler()
}
