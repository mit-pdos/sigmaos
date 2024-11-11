package beschedsrv

import (
	"time"
)

// Wait for the condition variable, or time out. Returns true if wait was
// succesful, or false if timed out. This function releases the lock.
//
// Caller holds lock.
func (pq *ProcQ) waitOrTimeoutAndUnlock() bool {
	ch := make(chan bool, 2)
	go func(ch chan bool) {
		time.Sleep(GET_PROC_TIMEOUT)
		ch <- false
	}(ch)
	go func(ch chan bool) {
		// Make sure to unlock, in the event that the waiting thread timed out.
		defer pq.mu.Unlock()
		pq.cond.Wait()
		ch <- true
	}(ch)
	return <-ch
}
