package crash

import (
	crand "crypto/rand"
	"log"
	"math/big"
	"os"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
)

//
// Crash/Delay testing
//

func MaybeDelay() {
	maxms := big.NewInt(1000)
	max := big.NewInt(1000)
	rr, _ := crand.Int(crand.Reader, max)
	if rr.Int64() < 660 {
		log.Printf("%v: DELAY %v\n", db.GetName(), proc.GetPid())
		// delay for a while.
		ms, _ := crand.Int(crand.Reader, maxms)
		time.Sleep(time.Duration(ms.Int64()) * time.Millisecond)
	}
}

func Crasher(fsl *fslib.FsLib) {
	maxms := big.NewInt(500)
	go func() {
		ms, _ := crand.Int(crand.Reader, maxms)
		time.Sleep(time.Duration(ms.Int64()) * time.Millisecond)
		max := big.NewInt(1000)
		rr, _ := crand.Int(crand.Reader, max)
		if rr.Int64() < 330 {
			// crash!
			log.Printf("%v: CRASH %v\n", db.GetName(), proc.GetPid())
			os.Exit(1)
		} else if rr.Int64() < 660 {
			log.Printf("%v: PARTITION %v\n", db.GetName(), proc.GetPid())
			fsl.Disconnect("name")
		}

	}()
}
