package debug

import (
	"fmt"
	"log"
	"os"
	"sync"
)

const (
	SERVICE = 10
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

// higher l, less debug output
func SetLevel(l int) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.level = l
}

func DPrintf(format string, v ...interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.debug {
		log.Printf("%v: %v", os.Args[0], fmt.Sprintf(format, v...))
	}
}

func DLPrintf(level int, format string, v ...interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if level <= db.level {
		log.Printf("%v: %v", os.Args[0], fmt.Sprintf(format, v...))
	}
}
