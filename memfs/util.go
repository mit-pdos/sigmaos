package memfs

import (
	"log"
)

var debug = false

func DPrintf(format string, a ...interface{}) {
	if debug {
		log.Printf(format, a...)
	}
}
