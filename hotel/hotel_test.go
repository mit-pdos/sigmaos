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
	p := proc.MakeProc(srv, []string{})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.Pid)
	assert.Nil(t, err, "WaitStarted")
	return p.Pid
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
	sts, err := ts.GetDir(np.DBD)
	assert.Nil(ts.T, err)
	assert.Equal(ts.T, 3, len(sts))
}

func TestGeo(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-geod"})
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELGEO)
	assert.Nil(t, err)
	arg := hotel.GeoRequest{
		Lat: 37.7749,
		Lon: -122.4194,
	}
	res := &hotel.GeoResult{}
	err = pdc.RPC("Geo.Nearby", arg, &res)
	assert.Nil(t, err)
	log.Printf("res %v\n", res)
	assert.Equal(t, 5, len(res.HotelIds))
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
	err = pdc.RPC("Rate.GetRates", arg, &res)
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
	err = pdc.RPC("Rec.GetRecs", arg, &res)
	assert.Nil(t, err)
	log.Printf("res %v\n", res.HotelIds)
	assert.Equal(t, 1, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestUser(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-userd"})
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELUSER)
	assert.Nil(t, err)
	arg := hotel.UserRequest{
		Name:     "u_0",
		Password: hotel.MkPassword("u_0"),
	}
	var res hotel.UserResult
	err = pdc.RPC("User.CheckUser", arg, &res)
	assert.Nil(t, err)
	log.Printf("res %v\n", res)
	ts.stop()
	ts.Shutdown()
}

func TestProfile(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-profd"})
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELPROF)
	assert.Nil(t, err)
	arg := hotel.ProfRequest{
		HotelIds: []string{"1", "2"},
	}
	var res hotel.ProfResult
	err = pdc.RPC("ProfSrv.GetProfiles", arg, &res)
	assert.Nil(t, err)
	for _, p := range res.Hotels {
		log.Printf("p %v\n", p)
	}
	assert.Equal(t, 2, len(res.Hotels))
	ts.stop()
	ts.Shutdown()
}

func TestCheck(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-reserved"})
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELRESERVE)
	assert.Nil(t, err)
	arg := hotel.ReserveRequest{
		HotelId:      []string{"1"},
		CustomerName: "u_0",
		InDate:       "2015-04-09",
		OutDate:      "2015-04-10",
		Number:       1,
	}
	var res hotel.ReserveResult
	err = pdc.RPC("Reserve.CheckAvailability", arg, &res)
	assert.Nil(t, err)
	log.Printf("res %v\n", res.HotelIds)
	ts.stop()
	ts.Shutdown()
}

func TestReserve(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-reserved"})
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELRESERVE)
	assert.Nil(t, err)
	arg := hotel.ReserveRequest{
		HotelId:      []string{"1"},
		CustomerName: "u_0",
		InDate:       "2015-04-09",
		OutDate:      "2015-04-10",
		Number:       1,
	}
	var res hotel.ReserveResult
	err = pdc.RPC("Reserve.MakeReservation", arg, &res)
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
	err = pdc.RPC("Search.Nearby", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestWww(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-userd", "user/hotel-rated",
		"user/hotel-geod", "user/hotel-profd", "user/hotel-searchd",
		"user/hotel-reserved", "user/hotel-recd", "user/hotel-wwwd"})

	for i := 0; i < 100; i++ {
		s, err := hotel.WebLogin("u_0", hotel.MkPassword("u_0"))
		assert.Nil(t, err)
		assert.Equal(t, "Login successfully!", s)

		err = hotel.WebSearch("2015-04-09", "2015-04-10", 37.7749, -122.4194)
		assert.Nil(t, err)

		err = hotel.WebRecs("dis", 38.0235, -122.095)
		assert.Nil(t, err)
	}
	s, err := hotel.WebReserve("2015-04-09", "2015-04-10", 38.0235, -122.095, "1", "u_0", "u_0", hotel.MkPassword("u_0"), 1)
	assert.Nil(t, err)
	assert.Equal(t, "Reserve successfully!", s)

	ts.stop()
	ts.Shutdown()
}
