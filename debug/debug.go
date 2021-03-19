package debug

import (
	"fmt"
	"log"
	"os"
	"sync"
)

type Debug struct {
	mu    sync.Mutex
	debug bool
	level int
}

var db Debug

func SetDebug(d bool) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if !db.debug {
		db.debug = d
	}
}

func DPrintf(format string, v ...interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.debug {
		log.Printf("%v: %v", os.Args[0], fmt.Sprintf(format, v...))
	}
}

func DLPrintf(src, label string, format string, v ...interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.debug {
		log.Printf("%v [%v]: %v", src, label, fmt.Sprintf(format, v...))
	}
}
