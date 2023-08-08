package delay

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/rand"
)

//
// Delay testing
//

var maxmsDelay int64

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
	db.DPrintf(db.DELAY, "Delay to %vms\n", ms)
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
