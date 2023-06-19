package named_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	// "sigmaos/groupmgr"
	// "sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestBootNamed(t *testing.T) {
	// crash := 1
	// crashinterval := 0

	ts := test.MakeTstateAll(t)

	sts, err1 := ts.GetDir(sp.NAMED + "/")
	assert.Nil(t, err1)
	log.Printf("named %v\n", sp.Names(sts))

	ts.Shutdown()
}

// type Tstate struct {
// 	*test.Tstate
// }

// func makeTstate(t *testing.T) *Tstate {
// 	ts := &Tstate{}
// 	ts.Tstate = test.MakeTstateAll(t)
// 	return ts
// }

// func TestNamedWalk(t *testing.T) {
// 	crash := 1
// 	crashinterval := 200
// 	// crashinterval := 0

// 	ts := makeTstate(t)

// 	pn := sp.NAMED + "/"

// 	d := []byte("hello")
// 	_, err := ts.PutFile(path.Join(pn, "testf"), 0777, sp.OWRITE, d)
// 	assert.Nil(t, err)

// 	ndg := startNamed(ts.SigmaClnt, "rootrealm", crash, crashinterval)

// 	// wait until kernel-started named exited and its lease expired
// 	time.Sleep((fsetcd.SessionTTL + 2) * time.Second)

// 	start := time.Now()
// 	i := 0
// 	for time.Since(start) < 10*time.Second {
// 		d1, err := ts.GetFile(path.Join(pn, "testf"))
// 		if err != nil {
// 			log.Printf("err %v\n", err)
// 			assert.Nil(t, err)
// 			break
// 		}
// 		assert.Equal(t, d, d1)
// 		i += 1
// 	}

// 	log.Printf("#getfile %d\n", i)

// 	for {
// 		err := ts.Remove(path.Join(pn, "testf"))
// 		if err == nil {
// 			break
// 		}
// 		log.Printf("remove f retry\n")
// 		time.Sleep(100 * time.Millisecond)
// 	}

// 	ndg.Stop()

// 	log.Printf("named stopped\n")

// 	ts.Shutdown()
// }
