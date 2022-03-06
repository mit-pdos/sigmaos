package delay

import (
	"time"

	db "ulambda/debug"
	"ulambda/proc"
	"ulambda/rand"
)

//
// Delay testing
//

var maxmsDelay int64
var totalDelay uint64
var lastTotal uint64

// If set RPCs maybe delayed by maxms
func SetDelayRPC(maxms int64) {
	maxmsDelay = maxms
}

func MaybeDelayRPC() {
	if maxmsDelay != 0 {
		r := rand.Int64(1000)
		if r < 660 {
			Delay(maxmsDelay)
		}
	}
}

func Delay(maxms int64) {
	ms := rand.Int64(maxms)
	totalDelay += ms
	if totalDelay-lastTotal > 1000 {
		lastTotal = totalDelay
	}
	db.DLPrintf("DELAY", "DELAY to %vms\n", totalDelay)
	time.Sleep(time.Duration(ms) * time.Millisecond)

}
