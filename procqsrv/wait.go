package procqsrv

import (
	"time"
)

// Wait for the condition variable, or time out. Returns true if wait was
// succesful, or false if timed out.
//
// Caller holds lock.
func (pq *ProcQ) waitOrTimeoutL() bool {
	ch := make(chan bool, 2)
	go func(ch chan bool) {
		time.Sleep(GET_PROC_TIMEOUT)
		ch <- false
	}(ch)
	go func(ch chan bool) {
		pq.cond.Wait()
		ch <- true
	}(ch)
	return <-ch
}
