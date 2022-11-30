package hotel_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	"sigmaos/clonedev"
	"sigmaos/dbd"
	db "sigmaos/debug"
	"sigmaos/hotel"
	"sigmaos/hotel/proto"
	"sigmaos/linuxsched"
	"sigmaos/loadgen"
	np "sigmaos/ninep"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	rd "sigmaos/rand"
	"sigmaos/sessdev"
	"sigmaos/test"
)

var K8S_ADDR string
var MAX_RPS int
var DURATION time.Duration

func init() {
	flag.StringVar(&K8S_ADDR, "k8saddr", "", "Addr of k8s frontend.")
	flag.IntVar(&MAX_RPS, "maxrps", 1000, "Max number of requests/sec.")
	flag.DurationVar(&DURATION, "duration", 10*time.Second, "Duration of load generation benchmarks.")
}

type Tstate struct {
	*test.Tstate
	job  string
	pids []proc.Tpid
	cc   *cacheclnt.CacheClnt
	cm   *cacheclnt.CacheMgr
}

func makeTstate(t *testing.T, srvs []string, ncache int) *Tstate {
	var err error
	ts := &Tstate{}
	ts.job = rd.String(8)
	ts.Tstate = test.MakeTstateAll(t)
	// If running as a test (not in a realm), and too few cores, then start more
	// procds.
	if !ts.RunningInRealm() {
		// Start enough procds to run all of the srvs and the caches.
		for i := 1; int(linuxsched.NCores)*i < len(srvs)+ncache; i++ {
			ts.BootProcd()
		}
	}
	ts.cc, ts.cm, ts.pids, err = hotel.MakeHotelJob(ts.FsLib, ts.ProcClnt, ts.job, srvs, 1, ncache)
	assert.Nil(ts.T, err)
	return ts
}

func (ts *Tstate) PrintStats() {
	for _, s := range np.HOTELSVC {
		ts.statsSrv(s)
	}
	cs, err := ts.cc.StatsSrv()
	assert.Nil(ts.T, err)
	for i, cstat := range cs {
		fmt.Printf("= cache-%v: %v\n", i, cstat)
	}
}

func (ts *Tstate) statsSrv(fn string) {
	stats := &protdevsrv.Stats{}
	err := ts.GetFileJson(fn+"/"+protdevsrv.STATS, stats)
	assert.Nil(ts.T, err, "error get stats %v", err)
	fmt.Printf("= %s: %v\n", fn, stats)
}

func (ts *Tstate) stop() {
	for _, pid := range ts.pids {
		err := ts.Evict(pid)
		assert.Nil(ts.T, err, "Evict: %v", err)
		_, err = ts.WaitExit(pid)
		assert.Nil(ts.T, err)
	}
	if ts.cm != nil {
		ts.cm.StopCache()
	}
	sts, err := ts.GetDir(np.DBD)
	assert.Nil(ts.T, err)
	assert.Equal(ts.T, 5, len(sts))
}

func TestGeoSingle(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-geod"}, 0)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELGEO)
	assert.Nil(t, err)
	arg := proto.GeoRequest{
		Lat: 37.7749,
		Lon: -122.4194,
	}
	res := proto.GeoResult{}
	err = pdc.RPC("Geo.Nearby", &arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.ALWAYS, "res %v\n", res)
	assert.Equal(t, 5, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestRateSingle(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-rated"}, hotel.NCACHE)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELRATE)
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
	ts := makeTstate(t, []string{"user/hotel-recd"}, 0)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELREC)
	assert.Nil(t, err)
	arg := &proto.RecRequest{
		Require: "dis",
		Lat:     38.0235,
		Lon:     -122.095,
	}
	var res proto.RecResult
	err = pdc.RPC("Rec.GetRecs", arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.ALWAYS, "res %v\n", res.HotelIds)
	assert.Equal(t, 1, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestUserSingle(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-userd"}, 0)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELUSER)
	assert.Nil(t, err)
	arg := &proto.UserRequest{
		Name:     "Cornell_0",
		Password: hotel.MkPassword("0"),
	}
	var res proto.UserResult
	err = pdc.RPC("User.CheckUser", arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.ALWAYS, "res %v\n", res)
	ts.stop()
	ts.Shutdown()
}

func TestProfile(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-profd"}, hotel.NCACHE)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELPROF)
	assert.Nil(t, err)
	arg := &proto.ProfRequest{
		HotelIds: []string{"1", "2"},
	}
	var res proto.ProfResult
	err = pdc.RPC("ProfSrv.GetProfiles", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Hotels))
	db.DPrintf(db.ALWAYS, "res %v\n", res.Hotels[0])

	err = pdc.RPC("ProfSrv.GetProfiles", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Hotels))

	ts.stop()
	ts.Shutdown()
}

func TestCheck(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-reserved"}, hotel.NCACHE)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELRESERVE)
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
	ts := makeTstate(t, []string{"user/hotel-reserved"}, hotel.NCACHE)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELRESERVE)
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

	b, err := ts.GetFile(np.DBD + clonedev.CloneName(dbd.QDEV))
	assert.Nil(t, err)
	q := fmt.Sprintf("select * from reservation")
	sidn := clonedev.SidName(string(b), dbd.QDEV)
	fn := np.DBD + sidn + "/" + sessdev.DataName(dbd.QDEV)
	_, err = ts.SetFile(fn, []byte(q), np.OWRITE, 0)
	assert.Nil(t, err)
	b, err = ts.GetFile(fn)
	assert.Nil(t, err)

	res := []hotel.Reservation{}
	err = json.Unmarshal(b, &res)
	assert.Nil(t, err)
	assert.Equal(t, "Alice", res[0].Customer)

	ts.Shutdown()
}

func TestSingleSearch(t *testing.T) {
	ts := makeTstate(t, []string{"user/hotel-geod", "user/hotel-rated", "user/hotel-searchd"}, hotel.NCACHE)
	pdc, err := protdevclnt.MkProtDevClnt(ts.FsLib, np.HOTELSEARCH)
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
	ts := makeTstate(t, []string{"user/hotel-userd",
		"user/hotel-rated", "user/hotel-geod", "user/hotel-profd",
		"user/hotel-searchd", "user/hotel-reserved", "user/hotel-recd",
		"user/hotel-wwwd"}, hotel.NCACHE)

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
	ts := makeTstate(t, hotel.HotelSvcs, hotel.NCACHE)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	hotel.RunDSB(t, 1000, wc)
	ts.PrintStats()
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
	// Write a file for clients to discover the server's address.
	p := hotel.JobHTTPAddrsPath(ts.job)
	if err := ts.PutFileJson(p, 0777, []string{K8S_ADDR}); err != nil {
		db.DFatalf("Error PutFileJson addrs %v", err)
	}
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	hotel.RunDSB(t, 1000, wc)
	ts.Shutdown()
}

func TestBenchSearch(t *testing.T) {
	ts := makeTstate(t, hotel.HotelSvcs, hotel.NCACHE)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	p := perf.MakePerf("TEST")
	defer p.Done()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func() {
		runSearch(ts.T, wc, r)
	})
	lg.Run()
	ts.PrintStats()
	ts.stop()
	ts.Shutdown()
}

func setupK8sState(ts *Tstate) {
	// Write a file for clients to discover the server's address.
	p := hotel.JobHTTPAddrsPath(ts.job)
	if err := ts.PutFileJson(p, 0777, []string{K8S_ADDR}); err != nil {
		db.DFatalf("Error PutFileJson addrs %v", err)
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
	pf := perf.MakePerf("TEST")
	defer pf.Done()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func() {
		runSearch(ts.T, wc, r)
	})
	lg.Run()
	ts.Shutdown()
}

func TestBenchGeo(t *testing.T) {
	ts := makeTstate(t, hotel.HotelSvcs, hotel.NCACHE)
	wc := hotel.MakeWebClnt(ts.FsLib, ts.job)
	p := perf.MakePerf("TEST")
	defer p.Done()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func() {
		runGeo(ts.T, wc, r)
	})
	lg.Run()
	ts.PrintStats()
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
	pf := perf.MakePerf("TEST")
	defer pf.Done()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	lg := loadgen.MakeLoadGenerator(DURATION, MAX_RPS, func() {
		runGeo(ts.T, wc, r)
	})
	lg.Run()
	ts.Shutdown()
}

func testMultiSearch(t *testing.T, nthread int) {
	const (
		N = 1000
	)
	ts := makeTstate(t, hotel.HotelSvcs, hotel.NCACHE)
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
	db.DPrintf(db.ALWAYS, "TestBenchMultiSearch nthread=%d N=%d %dms\n", nthread, N, time.Since(start).Milliseconds())
	ts.PrintStats()
	ts.stop()
	ts.Shutdown()
}

func TestMultiSearch(t *testing.T) {
	for _, n := range []int{1, 4} {
		testMultiSearch(t, n)
	}
}
