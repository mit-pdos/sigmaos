package hotel_test

import (
	"flag"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/dbclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/hotel"
	"sigmaos/hotel/proto"
	"sigmaos/linuxsched"
	"sigmaos/loadgen"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdev"
	"sigmaos/protdevclnt"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var K8S_ADDR string
var MAX_RPS int
var DURATION time.Duration
var cache string

const (
	NCACHESRV = 6
)

func init() {
	flag.StringVar(&K8S_ADDR, "k8saddr", "", "Addr of k8s frontend.")
	flag.IntVar(&MAX_RPS, "maxrps", 1000, "Max number of requests/sec.")
	flag.DurationVar(&DURATION, "duration", 10*time.Second, "Duration of load generation benchmarks.")
	flag.StringVar(&cache, "cache", "cached", "Cache service")
}

type Tstate struct {
	*test.Tstate
	job   string
	hotel *hotel.HotelJob
}

func makeTstate(t *testing.T, srvs []hotel.Srv, nshard int) *Tstate {
	var err error
	ts := &Tstate{}
	ts.job = rd.String(8)
	ts.Tstate = test.MakeTstateAll(t)
	n := 0
	for i := 1; int(linuxsched.NCores)*i < len(srvs)*2+nshard*2; i++ {
		n += 1
	}
	err = ts.BootNode(n)
	assert.Nil(ts.T, err)
	ts.hotel, err = hotel.MakeHotelJob(ts.SigmaClnt, ts.job, srvs, 80, cache, proc.Tcore(2), nshard, true, 0)
	assert.Nil(ts.T, err)
	return ts
}

func (ts *Tstate) PrintStats(lg *loadgen.LoadGenerator) {
	if lg != nil {
		lg.Stats()
	}
	for _, s := range sp.HOTELSVC {
		ts.statsSrv(s)
	}
	cs, err := ts.hotel.StatsSrv()
	assert.Nil(ts.T, err)
	for i, cstat := range cs {
		fmt.Printf("= cache-%v: %v\n", i, cstat)
	}
}

func (ts *Tstate) statsSrv(fn string) {
	stats := &protdev.StatInfo{}
	err := ts.GetFileJson(fn+"/"+protdev.STATS, stats)
	assert.Nil(ts.T, err, "error get stats %v", err)
	fmt.Printf("= %s: %v\n", fn, stats)
}

func (ts *Tstate) stop() {
	err := ts.hotel.Stop()
	assert.Nil(ts.T, err, "Stop: %v", err)
	sts, err := ts.GetDir(sp.DBD)
	assert.Nil(ts.T, err, "Error GetDir: %v", err)
	assert.Equal(ts.T, 5, len(sts), "%v", sts)
}

func TestGeoSingle(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-geod", Public: test.Overlays}}, 0)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELGEO)
	assert.Nil(t, err)
	arg := proto.GeoRequest{
		Lat: 37.7749,
		Lon: -122.4194,
	}
	res := proto.GeoResult{}
	err = pdc.RPC("Geo.Nearby", &arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "res %v\n", res.HotelIds)
	assert.Equal(t, 5, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestRateSingle(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-rated", Public: test.Overlays}}, NCACHESRV)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELRATE)
	assert.Nil(t, err)
	arg := &proto.RateRequest{
		HotelIds: []string{"5", "3", "1", "6", "2"}, // from TestGeo
		InDate:   "2015-04-09",
		OutDate:  "2015-04-10",
	}
	var res proto.RateResult
	err = pdc.RPC("Rate.GetRates", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.RatePlans))
	err = pdc.RPC("Rate.GetRates", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.RatePlans))
	ts.stop()
	ts.Shutdown()
}

func TestRecSingle(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-recd", Public: test.Overlays}}, 0)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELREC)
	assert.Nil(t, err)
	arg := &proto.RecRequest{
		Require: "dis",
		Lat:     38.0235,
		Lon:     -122.095,
	}
	var res proto.RecResult
	err = pdc.RPC("Rec.GetRecs", arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "res %v\n", res.HotelIds)
	assert.Equal(t, 1, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestUserSingle(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-userd", Public: test.Overlays}}, 0)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELUSER)
	assert.Nil(t, err)
	arg := &proto.UserRequest{
		Name:     "Cornell_0",
		Password: hotel.MkPassword("0"),
	}
	var res proto.UserResult
	err = pdc.RPC("User.CheckUser", arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "res %v\n", res)
	ts.stop()
	ts.Shutdown()
}

func TestProfile(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-profd", Public: test.Overlays}}, NCACHESRV)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELPROF)
	assert.Nil(t, err)
	arg := &proto.ProfRequest{
		HotelIds: []string{"1", "2"},
	}
	var res proto.ProfResult
	err = pdc.RPC("ProfSrv.GetProfiles", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Hotels))
	db.DPrintf(db.TEST, "res %v\n", res.Hotels[0])

	err = pdc.RPC("ProfSrv.GetProfiles", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Hotels))

	ts.stop()
	ts.Shutdown()
}

func TestCheck(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-reserved", Public: test.Overlays}}, NCACHESRV)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELRESERVE)
	assert.Nil(t, err)
	arg := &proto.ReserveRequest{
		HotelId:      []string{"4"},
		CustomerName: "Cornell_0",
		InDate:       "2015-04-09",
		OutDate:      "2015-04-10",
		Number:       1,
	}
	var res proto.ReserveResult
	err = pdc.RPC("Reserve.CheckAvailability", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))
	err = pdc.RPC("Reserve.CheckAvailability", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestReserve(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-reserved", Public: test.Overlays}}, NCACHESRV)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELRESERVE)
	assert.Nil(t, err)
	arg := &proto.ReserveRequest{
		HotelId:      []string{"4"},
		CustomerName: "Cornell_0",
		InDate:       "2015-04-09",
		OutDate:      "2015-04-10",
		Number:       1,
	}
	var res proto.ReserveResult

	err = pdc.RPC("Reserve.MakeReservation", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))

	err = pdc.RPC("Reserve.MakeReservation", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))

	ts.stop()
	ts.Shutdown()
}

func TestQueryDev(t *testing.T) {
	ts := test.MakeTstateAll(t)

	dbc, err := dbclnt.MkDbClnt(ts.FsLib, sp.DBD)
	assert.Nil(t, err)
	q := fmt.Sprintf("select * from reservation")
	res := []hotel.Reservation{}
	dbc.Query(q, &res)
	assert.Equal(t, "Alice", res[0].Customer)

	ts.Shutdown()
}

func TestSingleSearch(t *testing.T) {
	ts := makeTstate(t, []hotel.Srv{hotel.Srv{Name: "hotel-geod", Public: false}, hotel.Srv{Name: "hotel-rated", Public: false}, hotel.Srv{Name: "hotel-searchd", Public: test.Overlays}}, NCACHESRV)
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.FsLib}, sp.HOTELSEARCH)
	assert.Nil(t, err)
	arg := &proto.SearchRequest{
		Lat:     37.7749,
		Lon:     -122.4194,
		InDate:  "2015-04-09",
		OutDate: "2015-04-10",
	}
	var res proto.SearchResult
	err = pdc.RPC("Search.Nearby", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.HotelIds))
	err = pdc.RPC("Search.Nearby", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestWww(t *testing.T) {
	ts := makeTstate(t, hotel.MkHotelSvc(test.Overlays), NCACHESRV)

	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)

	s, err := wc.Login("Cornell_0", hotel.MkPassword("0"))
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)

	err = wc.Search("2015-04-09", "2015-04-10", 37.7749, -122.4194)
	assert.Nil(t, err)

	err = wc.Recs("dis", 38.0235, -122.095)
	assert.Nil(t, err)

	s, err = wc.Reserve("2015-04-09", "2015-04-10", 38.0235, -122.095, "1", "Cornell_0", "Cornell_0", hotel.MkPassword("0"), 1)
	assert.Nil(t, err)
	assert.Equal(t, "Reserve successfully!", s)

	s, err = wc.Geo(37.7749, -122.4194)
	assert.Nil(t, err)
	assert.Equal(t, "Geo!", s)

	ts.stop()
	ts.Shutdown()
}

func runSearch(t *testing.T, wc *hotel.WebClnt, r *rand.Rand) {
	err := hotel.RandSearchReq(wc, r)
	assert.Nil(t, err, "Err search %v", err)
}

func runRecommend(t *testing.T, wc *hotel.WebClnt, r *rand.Rand) {
	err := hotel.RandRecsReq(wc, r)
	assert.Nil(t, err)
}

func runLogin(t *testing.T, wc *hotel.WebClnt, r *rand.Rand) {
	s, err := hotel.RandLoginReq(wc, r)
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)
}

func runReserve(t *testing.T, wc *hotel.WebClnt, r *rand.Rand) {
	s, err := hotel.RandReserveReq(wc, r)
	assert.Nil(t, err)
	assert.Equal(t, "Reserve successfully!", s)
}

func runGeo(t *testing.T, wc *hotel.WebClnt, r *rand.Rand) {
	s, err := hotel.GeoReq(wc)
	assert.Nil(t, err, "Err geo %v", err)
	assert.Equal(t, "Geo!", s)
}

func TestBenchDeathStarSingle(t *testing.T) {
	ts := makeTstate(t, hotel.MkHotelSvc(test.Overlays), NCACHESRV)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	hotel.RunDSB(t, 1000, wc, r)
	//ts.PrintStats(nil)
	ts.stop()
	ts.Shutdown()
}

func TestBenchDeathStarSingleK8s(t *testing.T) {
	// Bail out if no addr was provided.
	if K8S_ADDR == "" {
		db.DPrintf(db.ALWAYS, "No k8s addr supplied")
		return
	}
	ts := makeTstate(t, nil, 0)

	setupK8sState(ts)

	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	hotel.RunDSB(t, 1000, wc, r)
	ts.Shutdown()
}

func TestBenchSearchSigma(t *testing.T) {
	ts := makeTstate(t, hotel.MkHotelSvc(test.Overlays), NCACHESRV)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	p, err := perf.MakePerf(perf.TEST)
	assert.Nil(t, err)
	defer p.Done()
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) {
		runSearch(ts.T, wc, r)
	})
	lg.Calibrate()
	lg.Run()
	ts.PrintStats(lg)
	ts.stop()
	ts.Shutdown()
}

func setupK8sState(ts *Tstate) {
	// Advertise server address
	p := hotel.JobHTTPAddrsPath(ts.job)
	mnt := sp.MkMountService(sp.MkTaddrs([]string{K8S_ADDR}))
	if err := ts.MountService(p, mnt); err != nil {
		db.DFatalf("MountService %v", err)
	}
}

func TestBenchSearchK8s(t *testing.T) {
	// Bail out if no addr was provided.
	if K8S_ADDR == "" {
		db.DPrintf(db.ALWAYS, "No k8s addr supplied")
		return
	}
	ts := makeTstate(t, nil, 0)
	setupK8sState(ts)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	pf, err := perf.MakePerf(perf.TEST)
	assert.Nil(t, err)
	defer pf.Done()
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) {
		runSearch(ts.T, wc, r)
	})
	lg.Calibrate()
	lg.Run()
	ts.Shutdown()
}

func TestBenchGeoSigma(t *testing.T) {
	ts := makeTstate(t, hotel.MkHotelSvc(test.Overlays), NCACHESRV)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	p, err := perf.MakePerf(perf.TEST)
	assert.Nil(t, err)
	defer p.Done()
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) {
		runGeo(ts.T, wc, r)
	})
	lg.Calibrate()
	lg.Run()
	ts.PrintStats(lg)
	ts.stop()
	ts.Shutdown()
}

func TestBenchGeoK8s(t *testing.T) {
	// Bail out if no addr was provided.
	if K8S_ADDR == "" {
		db.DPrintf(db.ALWAYS, "No k8s addr supplied")
		return
	}
	ts := makeTstate(t, nil, 0)
	setupK8sState(ts)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	pf, err := perf.MakePerf(perf.TEST)
	assert.Nil(t, err)
	defer pf.Done()
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) {
		runGeo(ts.T, wc, r)
	})
	lg.Calibrate()
	lg.Run()
	ts.Shutdown()
}

func testMultiSearch(t *testing.T, nthread int) {
	const (
		N = 1000
	)
	ts := makeTstate(t, hotel.MkHotelSvc(test.Overlays), NCACHESRV)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	ch := make(chan bool)
	start := time.Now()
	for t := 0; t < nthread; t++ {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		go func() {
			for i := 0; i < N; i++ {
				runSearch(ts.T, wc, r)
			}
			ch <- true
		}()
	}
	for t := 0; t < nthread; t++ {
		<-ch
	}
	db.DPrintf(db.TEST, "TestBenchMultiSearch nthread=%d N=%d %dms\n", nthread, N, time.Since(start).Milliseconds())
	ts.PrintStats(nil)
	ts.stop()
	ts.Shutdown()
}

func TestMultiSearch(t *testing.T) {
	for _, n := range []int{1, 4} {
		testMultiSearch(t, n)
	}
}
