package debug

import (
	"fmt"
	"log"
	"os"
)

var debug bool

func SetDebug(d bool) {
	if !debug {
		debug = d
	}

}

func DPrintf(format string, v ...interface{}) {
	if debug {
		log.Printf("%v: %v", os.Args[0], fmt.Sprintf(format, v...))
	}
}
