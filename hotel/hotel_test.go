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
	pids []proc.Tpid
}

func spawn(t *testing.T, ts *Tstate, srv string) proc.Tpid {
	a := proc.MakeProc(srv, []string{})
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
	return a.Pid
}

func makeTstate(t *testing.T, srvs []string) *Tstate {
	var err error
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.pids = make([]proc.Tpid, 0)
	for _, s := range srvs {
		pid := spawn(t, ts, s)
		err = ts.WaitStart(pid)
		assert.Nil(t, err)
		ts.pids = append(ts.pids, pid)
	}
	return ts
}

func (ts *Tstate) stop() {
	for _, pid := range ts.pids {
		err := ts.Evict(pid)
		assert.Nil(ts.T, err, "Evict: %v", err)
		_, err = ts.WaitExit(pid)
		assert.Nil(ts.T, err)
	}
}

func TestGeo(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-geod"})
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
	ts.stop()
	ts.Shutdown()
}

func TestRate(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-rated"})
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
	ts.stop()
	ts.Shutdown()
}

func TestRec(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-recd"})
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELREC)
	assert.Nil(t, err)
	arg := hotel.RecRequest{
		Require: "dis",
		Lat:     38.0235,
		Lon:     -122.095,
	}
	var res hotel.RecResult
	err = pdc.RPCJson(&arg, &res)
	assert.Nil(t, err)
	log.Printf("res %v\n", res.HotelIds)
	ts.stop()
	ts.Shutdown()
}

func TestSearch(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-geod", "user/hotel-rated", "user/hotel-searchd"})
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELSEARCH)
	assert.Nil(t, err)
	arg := hotel.SearchRequest{
		Lat:     37.7749,
		Lon:     -122.4194,
		InDate:  "2015-04-09",
		OutDate: "2015-04-10",
	}
	var res hotel.SearchResult
	err = pdc.RPCJson(&arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}
