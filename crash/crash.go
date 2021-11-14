package crash

import (
	crand "crypto/rand"
	"math/big"
	"os"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
)

//
// Crash/partition testing
//

func Crasher(fsl *fslib.FsLib) {
	maxms := big.NewInt(500)
	go func() {
		ms, _ := crand.Int(crand.Reader, maxms)
		time.Sleep(time.Duration(ms.Int64()) * time.Millisecond)
		max := big.NewInt(1000)
		rr, _ := crand.Int(crand.Reader, max)
		if rr.Int64() < 330 {
			// crash!
			db.DPrintf("%v: CRASH %v\n", db.GetName(), proc.GetPid())
			os.Exit(1)
		} else if rr.Int64() < 660 {
			db.DPrintf("%v: PARTITION %v\n", db.GetName(), proc.GetPid())
			fsl.Disconnect("name")
		}

	}()
}
