package hotel_test

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"testing"
	"time"

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

func (ts *Tstate) Stats(fn string) {
	b, err := ts.GetFile(fn + "/stats")
	assert.Nil(ts.T, err)
	fmt.Printf("stats %s: %v\n", fn, string(b))
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
	assert.Equal(ts.T, 4, len(sts))
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

	s, err := hotel.WebLogin("u_0", hotel.MkPassword("u_0"))
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)

	err = hotel.WebSearch("2015-04-09", "2015-04-10", 37.7749, -122.4194)
	assert.Nil(t, err)

	err = hotel.WebRecs("dis", 38.0235, -122.095)
	assert.Nil(t, err)

	s, err = hotel.WebReserve("2015-04-09", "2015-04-10", 38.0235, -122.095, "1", "u_0", "u_0", hotel.MkPassword("u_0"), 1)
	assert.Nil(t, err)
	assert.Equal(t, "Reserve successfully!", s)

	ts.stop()
	ts.Shutdown()
}

func benchSearch(t *testing.T, r *rand.Rand) {
	in_date := r.Intn(14) + 9
	out_date := in_date + r.Intn(5) + 1
	in_date_str := fmt.Sprintf("2015-04-%d", in_date)
	if in_date <= 9 {
		in_date_str = fmt.Sprintf("2015-04-0%d", in_date)
	}
	out_date_str := fmt.Sprintf("2015-04-%d", out_date)
	if out_date <= 9 {
		out_date_str = fmt.Sprintf("2015-04-0%d", out_date)
	}
	lat := 38.0235 + (float64(r.Intn(481))-240.5)/1000.0
	lon := -122.095 + (float64(r.Intn(325))-157.0)/1000.0
	err := hotel.WebSearch(in_date_str, out_date_str, lat, lon)
	assert.Nil(t, err)
}

func benchRecommend(t *testing.T, r *rand.Rand) {
	coin := toss(r)
	req := ""
	if coin < 0.33 {
		req = "dis"
	} else if coin < 0.66 {
		req = "rate"
	} else {
		req = "price"
	}
	lat := 38.0235 + (float64(r.Intn(481))-240.5)/1000.0
	lon := -122.095 + (float64(r.Intn(325))-157.0)/1000.0
	err := hotel.WebRecs(req, lat, lon)
	assert.Nil(t, err)
}

func benchLogin(t *testing.T, r *rand.Rand) {
	user := fmt.Sprintf("u_%d", r.Intn(500))
	pw := hotel.MkPassword(user)
	s, err := hotel.WebLogin(user, pw)
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)
}

func benchReserve(t *testing.T, r *rand.Rand) {
	in_date := r.Intn(14) + 9
	out_date := in_date + r.Intn(5) + 1
	in_date_str := fmt.Sprintf("2015-04-%d", in_date)
	if in_date <= 9 {
		in_date_str = fmt.Sprintf("2015-04-0%d", in_date)
	}
	out_date_str := fmt.Sprintf("2015-04-%d", out_date)
	if out_date <= 9 {
		out_date_str = fmt.Sprintf("2015-04-0%d", out_date)
	}
	hotelid := strconv.Itoa(r.Intn(80) + 1)
	user := fmt.Sprintf("u_%d", r.Intn(500))
	pw := hotel.MkPassword(user)
	cust_name := user
	num := 1
	lat := 38.0235 + (float64(r.Intn(481))-240.5)/1000.0
	lon := -122.095 + (float64(r.Intn(325))-157.0)/1000.0
	s, err := hotel.WebReserve(in_date_str, out_date_str, lat, lon, hotelid, user, cust_name, pw, num)
	assert.Nil(t, err)
	assert.Equal(t, "Reserve successfully!", s)
}

func toss(r *rand.Rand) float64 {
	toss := r.Intn(1000)
	return float64(toss) / 1000
}

var hotelsvcs = []string{"user/hotel-userd", "user/hotel-rated",
	"user/hotel-geod", "user/hotel-profd", "user/hotel-searchd",
	"user/hotel-reserved", "user/hotel-recd", "user/hotel-wwwd"}

func TestBenchDeathStarSingle(t *testing.T) {
	const (
		N               = 1000
		search_ratio    = 0.6
		recommend_ratio = 0.39
		user_ratio      = 0.005
		reserve_ratio   = 0.005
	)
	ts := makeTstate(t, hotelsvcs)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	start := time.Now()
	for i := 0; i < N; i++ {
		coin := toss(r)
		if coin < search_ratio {
			benchSearch(t, r)
		} else if coin < search_ratio+recommend_ratio {
			benchRecommend(t, r)
		} else if coin < search_ratio+recommend_ratio+user_ratio {
			benchLogin(t, r)
		} else {
			benchReserve(t, r)
		}
	}
	fmt.Printf("TestBenchDeathStarSingle N=%d %dms\n", N, time.Since(start).Milliseconds())
	for _, s := range np.HOTELSVC {
		ts.Stats(s)
	}
	ts.stop()
	ts.Shutdown()
}

func testMultiSearch(t *testing.T, nthread int) {
	const (
		N = 1000
	)
	ts := makeTstate(t, hotelsvcs)
	ch := make(chan bool)
	start := time.Now()
	for t := 0; t < nthread; t++ {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		go func() {
			for i := 0; i < N; i++ {
				benchSearch(ts.T, r)
			}
			ch <- true
		}()
	}
	for t := 0; t < nthread; t++ {
		<-ch
	}
	fmt.Printf("TestBenchMultiSearch nthread=%d N=%d %dms\n", nthread, N, time.Since(start).Milliseconds())
	for _, s := range np.HOTELSVC {
		ts.Stats(s)
	}
	ts.stop()
	ts.Shutdown()
}

func TestMultiSearch(t *testing.T) {
	for n := 5; n < 6; n++ {
		testMultiSearch(t, n)
	}
}
