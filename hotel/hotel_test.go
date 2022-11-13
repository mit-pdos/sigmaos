package hotel_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	"sigmaos/hotel"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	pid proc.Tpid
}

func spawn(t *testing.T, ts *Tstate) proc.Tpid {
	a := proc.MakeProc("user/geod", []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return a.Pid
}

func makeTstate(t *testing.T) *Tstate {
	var err error
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.pid = spawn(t, ts)
	err = ts.WaitStart(ts.pid)
	assert.Nil(t, err)
	return ts
}

func (ts *Tstate) stop(pid proc.Tpid) {
	err := ts.Evict(pid)
	assert.Nil(ts.T, err, "Evict: %v", err)
	_, err = ts.WaitExit(pid)
	assert.Nil(ts.T, err)
}

func TestGeo(t *testing.T) {
	ts := makeTstate(t)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELGEO)
	assert.Nil(t, err)
	arg := hotel.GeoRequest{
		Lat: 37.7749,
		Lon: -122.4194,
	}
	var res hotel.GeoResult
	err = pdc.RPCJson(&arg, &res)
	assert.Nil(t, err)
	log.Printf("res %v\n", res)
	ts.stop(ts.pid)
	ts.Shutdown()
}
