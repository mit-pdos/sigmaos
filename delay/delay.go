package delay

import (
	crand "crypto/rand"
	"math/big"
	"time"

	db "ulambda/debug"
	"ulambda/proc"
)

//
// Delay testing
//

var maxmsDelay int64
var totalDelay int64
var lastTotal int64

func SetDelay(maxms int64) {
	maxmsDelay = maxms
}

func Delay() {
	max := big.NewInt(maxmsDelay)
	ms, _ := crand.Int(crand.Reader, max)
	totalDelay += ms.Int64()
	if totalDelay-lastTotal > 1000 {
		lastTotal = totalDelay
	}
	db.DPrintf("%v: DELAY %v tot %vms\n", db.GetName(), proc.GetPid(), totalDelay)
	time.Sleep(time.Duration(ms.Int64()) * time.Millisecond)

}

func MaybeDelay() {
	if maxmsDelay != 0 {
		max := big.NewInt(1000)
		rr, _ := crand.Int(crand.Reader, max)
		if rr.Int64() < 660 {
			Delay()
		}
	}
}
