package srv

import (
	"time"

	sp "sigmaos/sigmap"
)

func (be *BESched) wakeupWaiters() {
	for {
		time.Sleep(5 * sp.Conf.BESched.GET_PROC_TIMEOUT)
		be.mu.Lock()
		be.cond.Broadcast()
		be.mu.Unlock()
	}
}

// Wait for the condition variable, or time out. Returns true if wait was
// succesful, or false if timed out. This function releases the lock.
//
// Caller holds lock.
func (be *BESched) waitOrTimeoutAndUnlock(nEnqueues uint64) bool {
	waitUntil := time.Now().Add(sp.Conf.BESched.GET_PROC_TIMEOUT)
	ch := make(chan bool, 2)
	go func(ch chan bool) {
		time.Sleep(time.Until(waitUntil))
		ch <- false
	}(ch)
	go func(ch chan bool) {
		// Make sure to unlock, in the event that the waiting thread timed out.
		defer be.mu.Unlock()
		timedOut := false
		// Wait until something is enqueued or we time out.
		for be.nEnqueues == nEnqueues {
			if time.Now().After(waitUntil) {
				timedOut = true
				break
			}
			be.cond.Wait()
		}
		ch <- timedOut
	}(ch)
	return <-ch
}
