package test

import (
	"math/rand"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
)

// Sleep for a random time, then crash a server.  Boot a server of a
// certain type, then crash a server of that type.
func (ts *Tstate) CrashServer(srv string, randMax int, l *sync.Mutex, crashchan chan bool) {
	r := rand.Intn(randMax)
	time.Sleep(time.Duration(r) * time.Microsecond)
	db.DPrintf(db.ALWAYS, "Crashing a %v after %v", srv, time.Duration(r)*time.Microsecond)
	// Make sure not too many crashes happen at once by taking a lock (we always
	// want >= 1 server to be up).
	l.Lock()
	db.DPrintf(db.ALWAYS, "Booting a node Before crashing a %v.", srv)
	err := ts.BootNode(1)
	db.DPrintf(db.ALWAYS, "Done booting a node before crashing a %v.", srv)
	if !assert.Nil(ts.T, err) {
		db.DPrintf(db.ERROR, "Error BootNode %v", srv)
	}
	db.DPrintf(db.ALWAYS, "Kill one %v", srv)
	err = ts.KillOne(srv)
	if !assert.Nil(ts.T, err) {
		db.DPrintf(db.ERROR, "Error non-nil kill %v: %v", srv, err)
	}
	db.DPrintf(db.ALWAYS, "Done Kill one %v", srv)
	l.Unlock()
	crashchan <- true
}
