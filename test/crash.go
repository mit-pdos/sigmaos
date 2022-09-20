package test

import (
	"log"
	"math/rand"
	"sync"
	"time"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/proc"
)

// Sleep for a random time, then crash a server.  Crash a server of a
// certain type, then crash a server of that type.
func (ts *Tstate) CrashServer(srv string, randMax int, l *sync.Mutex, crashchan chan bool) {
	r := rand.Intn(randMax)
	time.Sleep(time.Duration(r) * time.Microsecond)
	log.Printf("Crashing a %v after %v", srv, time.Duration(r)*time.Microsecond)
	// Make sure not too many crashes happen at once by taking a lock (we always
	// want >= 1 server to be up).
	l.Lock()
	switch srv {
	case np.PROCD:
		err := ts.BootProcd()
		if err != nil {
			db.DFatalf("Error spawn procd")
		}
	case np.UX:
		err := ts.BootFsUxd()
		if err != nil {
			db.DFatalf("Error spawn uxd")
		}
	default:
		db.DFatalf("%v: Unrecognized service type", proc.GetProgram())
	}
	log.Printf("Kill one %v", srv)
	err := ts.KillOne(srv)
	if err != nil {
		db.DFatalf("Error non-nil kill procd: %v", err)
	}
	l.Unlock()
	crashchan <- true
}
