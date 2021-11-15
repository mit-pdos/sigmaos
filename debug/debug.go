package debug

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
)

type Debug struct {
	mu    sync.Mutex
	debug bool
	name  string
}

var db Debug

func Name(n string) {
	uldebug := os.Getenv("SIGMADEBUG")

	db.mu.Lock()
	defer db.mu.Unlock()
	if uldebug != "" {
		db.debug = true
	}
	db.name = n + ":" + strconv.Itoa(os.Getpid())
}

func GetName() string {
	return db.name
}

func DPrintf(format string, v ...interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.debug {
		log.Printf("%v: %v", os.Args[0], fmt.Sprintf(format, v...))
	}
}

func DLPrintf(label string, format string, v ...interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.debug {
		log.Printf("%v %v %v", db.name, label, fmt.Sprintf(format, v...))
	}
}
