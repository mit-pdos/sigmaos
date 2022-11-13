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

func spawn(t *testing.T, ts *Tstate, srv string) proc.Tpid {
	a := proc.MakeProc(srv, []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return a.Pid
}

func makeTstate(t *testing.T, srv string) *Tstate {
	var err error
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.pid = spawn(t, ts, srv)
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
	ts := makeTstate(t, "user/geod")
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

func TestRate(t *testing.T) {
	ts := makeTstate(t, "user/rated")
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELRATE)
	assert.Nil(t, err)
	arg := hotel.RateRequest{
		HotelIds: []string{"5", "3", "1", "6", "2"}, // from TestGeo
		InDate:   "2015-04-09",
		OutDate:  "2015-04-10",
	}
	var res hotel.RateResult
	err = pdc.RPCJson(&arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.RatePlans))
	ts.stop(ts.pid)
	ts.Shutdown()
}
