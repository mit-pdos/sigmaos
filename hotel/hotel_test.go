package hotel_test

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"path"
	"strconv"
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
	rd "sigmaos/rand"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var K8S_ADDR string
var MAX_RPS int
var DURATION time.Duration
var cache string
var TEST_AUTH bool

const (
	NCACHESRV = 6
)

func init() {
	flag.StringVar(&K8S_ADDR, "k8saddr", "", "Addr of k8s frontend.")
	flag.IntVar(&MAX_RPS, "maxrps", 1000, "Max number of requests/sec.")
	flag.BoolVar(&TEST_AUTH, "auth", false, "Testing k8s auth")
	flag.DurationVar(&DURATION, "duration", 10*time.Second, "Duration of load generation benchmarks.")
	flag.StringVar(&cache, "cache", "cached", "Cache service")
}

type Tstate struct {
	*test.Tstate
	job   string
	hotel *hotel.HotelJob
}

func newTstate(t1 *test.Tstate, srvs []hotel.Srv, nserver int) *Tstate {
	var err error
	ts := &Tstate{}
	ts.job = rd.String(8)
	ts.Tstate = t1
	n := 0
	for i := 1; int(linuxsched.GetNCores())*i < len(srvs)*2+nserver*2; i++ {
		n += 1
	}
	err = ts.BootNode(n)
	assert.Nil(ts.T, err)
	ts.hotel, err = hotel.NewHotelJob(ts.SigmaClnt, ts.job, srvs, 80, cache, proc.Tmcpu(2000), nserver, true, 0)
	assert.Nil(ts.T, err)
	return ts
}

func (ts *Tstate) PrintStats(lg *loadgen.LoadGenerator) {
	if lg != nil {
		lg.Stats()
	}
	for _, s := range hotel.HOTELSVC {
		ts.statsSrv(s)
	}
	cs, err := ts.hotel.StatsSrv()
	assert.Nil(ts.T, err)
	for i, cstat := range cs {
		fmt.Printf("= cache-%v: %v\n", i, cstat)
	}
}

func (ts *Tstate) statsSrv(fn string) {
	stats := &rpc.SigmaRPCStats{}
	pn := path.Join(fn, rpc.RPC, rpc.STATS)
	err := ts.GetFileJson(pn, stats)
	assert.Nil(ts.T, err, "error get stats %v", err)
	fmt.Printf("= %s: %v\n", pn, stats)
}

func (ts *Tstate) stop() {
	err := ts.hotel.Stop()
	assert.Nil(ts.T, err, "Stop: %v", err)
	sts, err := ts.GetDir(sp.DBD)
	assert.Nil(ts.T, err, "Error GetDir: %v", err)
	assert.True(ts.T, len(sts) < 10)
}

func TestCompile(t *testing.T) {
}

func TestGeoSingle(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-geod", Public: test.Overlays}}, 0)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELGEO)
	assert.Nil(t, err)
	arg := proto.GeoRequest{
		Lat: 37.7749,
		Lon: -122.4194,
	}
	res := proto.GeoResult{}
	err = rpcc.RPC("Geo.Nearby", &arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "res %v\n", res.HotelIds)
	assert.Equal(t, 5, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestRateSingle(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-rated", Public: test.Overlays}}, NCACHESRV)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELRATE)
	assert.Nil(t, err)
	arg := &proto.RateRequest{
		HotelIds: []string{"5", "3", "1", "6", "2"}, // from TestGeo
		InDate:   "2015-04-09",
		OutDate:  "2015-04-10",
	}
	var res proto.RateResult
	err = rpcc.RPC("Rate.GetRates", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.RatePlans))
	err = rpcc.RPC("Rate.GetRates", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.RatePlans))
	ts.stop()
	ts.Shutdown()
}

func TestRecSingle(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-recd", Public: test.Overlays}}, 0)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELREC)
	assert.Nil(t, err)
	arg := &proto.RecRequest{
		Require: "dis",
		Lat:     38.0235,
		Lon:     -122.095,
	}
	var res proto.RecResult
	err = rpcc.RPC("Rec.GetRecs", arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "res %v\n", res.HotelIds)
	assert.Equal(t, 1, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestUserSingle(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-userd", Public: test.Overlays}}, 0)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELUSER)
	assert.Nil(t, err)
	arg := &proto.UserRequest{
		Name:     "Cornell_0",
		Password: hotel.NewPassword("0"),
	}
	var res proto.UserResult
	err = rpcc.RPC("Users.CheckUser", arg, &res)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "res %v\n", res)
	ts.stop()
	ts.Shutdown()
}

func TestProfile(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-profd", Public: test.Overlays}}, NCACHESRV)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELPROF)
	assert.Nil(t, err)
	arg := &proto.ProfRequest{
		HotelIds: []string{"1", "2"},
	}
	var res proto.ProfResult
	err = rpcc.RPC("ProfSrv.GetProfiles", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Hotels))
	db.DPrintf(db.TEST, "res %v\n", res.Hotels[0])

	err = rpcc.RPC("ProfSrv.GetProfiles", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Hotels))

	ts.stop()
	ts.Shutdown()
}

func TestCheck(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-reserved", Public: test.Overlays}}, NCACHESRV)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELRESERVE)
	assert.Nil(t, err)
	arg := &proto.ReserveRequest{
		HotelId:      []string{"4"},
		CustomerName: "Cornell_0",
		InDate:       "2015-04-09",
		OutDate:      "2015-04-10",
		Number:       1,
	}
	var res proto.ReserveResult
	err = rpcc.RPC("Reserve.CheckAvailability", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))
	err = rpcc.RPC("Reserve.CheckAvailability", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestReserve(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-reserved", Public: test.Overlays}}, NCACHESRV)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELRESERVE)
	assert.Nil(t, err)
	arg := &proto.ReserveRequest{
		HotelId:      []string{"4"},
		CustomerName: "Cornell_0",
		InDate:       "2015-04-09",
		OutDate:      "2015-04-10",
		Number:       1,
	}
	var res proto.ReserveResult

	err = rpcc.RPC("Reserve.NewReservation", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))

	err = rpcc.RPC("Reserve.NewReservation", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.HotelIds))

	ts.stop()
	ts.Shutdown()
}

func TestQueryDev(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dbc, err := dbclnt.NewDbClnt(ts.FsLib, sp.DBD)
	assert.Nil(t, err)
	q := fmt.Sprintf("select * from reservation")
	res := []hotel.Reservation{}
	err = dbc.Query(q, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res))

	ts.Shutdown()
}

func TestSingleSearch(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, []hotel.Srv{hotel.Srv{Name: "hotel-geod", Public: false}, hotel.Srv{Name: "hotel-rated", Public: false}, hotel.Srv{Name: "hotel-searchd", Public: test.Overlays}}, NCACHESRV)
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, hotel.HOTELSEARCH)
	assert.Nil(t, err)
	arg := &proto.SearchRequest{
		Lat:     37.7749,
		Lon:     -122.4194,
		InDate:  "2015-04-09",
		OutDate: "2015-04-10",
	}
	var res proto.SearchResult
	err = rpcc.RPC("Search.Nearby", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.HotelIds))
	err = rpcc.RPC("Search.Nearby", arg, &res)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res.HotelIds))
	ts.stop()
	ts.Shutdown()
}

func TestWww(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, hotel.NewHotelSvc(test.Overlays), NCACHESRV)

	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)

	s, err := wc.Login("Cornell_0", hotel.NewPassword("0"))
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)

	err = wc.Search("2015-04-09", "2015-04-10", 37.7749, -122.4194)
	assert.Nil(t, err)

	err = wc.Recs("dis", 38.0235, -122.095)
	assert.Nil(t, err)

	s, err = wc.Reserve("2015-04-09", "2015-04-10", 38.0235, -122.095, "1", "Cornell_0", "Cornell_0", hotel.NewPassword("0"), 1)
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
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, hotel.NewHotelSvc(test.Overlays), NCACHESRV)
	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
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
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nil, 0)

	err1 = setupK8sState(ts)
	assert.Nil(t, err1, "Error setupK8sState: %v", err1)

	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	hotel.RunDSB(t, 1000, wc, r)
	ts.Shutdown()
}

func TestBenchSearchSigma(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, hotel.NewHotelSvc(test.Overlays), NCACHESRV)
	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
	p, err := perf.NewPerf(ts.ProcEnv(), perf.TEST)
	assert.Nil(t, err)
	defer p.Done()
	lg := loadgen.NewLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) (time.Duration, bool) {
		runSearch(ts.T, wc, r)
		return 0, false
	})
	lg.Calibrate()
	lg.Run()
	ts.PrintStats(lg)
	ts.stop()
	ts.Shutdown()
}

func setupK8sState(ts *Tstate) error {
	// Advertise server address
	p := hotel.JobHTTPAddrsPath(ts.job)
	h, po, err := net.SplitHostPort(K8S_ADDR)
	assert.Nil(ts.T, err, "Err split host port %v: %v", K8S_ADDR, err)
	port, err := strconv.Atoi(po)
	assert.Nil(ts.T, err, "Err parse port %v: %v", po, err)
	addr := sp.NewTaddrRealm(sp.Tip(h), sp.INNER_CONTAINER_IP, sp.Tport(port), ts.ProcEnv().GetNet())
	mnt := sp.NewMountService([]*sp.Taddr{addr})
	err = ts.MkMountFile(p, mnt, sp.NoLeaseId)
	if !assert.Nil(ts.T, err) {
		db.DPrintf(db.ERROR, "MkMountFile %v", err)
		return err
	}
	return nil
}

func TestBenchSearchK8s(t *testing.T) {
	// Bail out if no addr was provided.
	if K8S_ADDR == "" {
		db.DPrintf(db.ALWAYS, "No k8s addr supplied")
		return
	}
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nil, 0)
	err1 = setupK8sState(ts)
	assert.Nil(t, err1, "Error setupK8sState: %v", err1)
	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
	pf, err := perf.NewPerf(ts.ProcEnv(), perf.TEST)
	assert.Nil(t, err)
	defer pf.Done()
	lg := loadgen.NewLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) (time.Duration, bool) {
		runSearch(ts.T, wc, r)
		return 0, false
	})
	lg.Calibrate()
	lg.Run()
	ts.Shutdown()
}

func TestBenchGeoSigma(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, hotel.NewHotelSvc(test.Overlays), NCACHESRV)
	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
	p, err := perf.NewPerf(ts.ProcEnv(), perf.TEST)
	assert.Nil(t, err)
	defer p.Done()
	lg := loadgen.NewLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) (time.Duration, bool) {
		runGeo(ts.T, wc, r)
		return 0, false
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
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nil, 0)
	err1 = setupK8sState(ts)
	assert.Nil(t, err1, "Error setupK8sState: %v", err1)
	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
	pf, err := perf.NewPerf(ts.ProcEnv(), perf.TEST)
	assert.Nil(t, err)
	defer pf.Done()
	lg := loadgen.NewLoadGenerator(DURATION, MAX_RPS, func(r *rand.Rand) (time.Duration, bool) {
		runGeo(ts.T, wc, r)
		return 0, false
	})
	lg.Calibrate()
	lg.Run()
	ts.Shutdown()
}

func testMultiSearch(t *testing.T, nthread int) {
	const (
		N = 1000
	)
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, hotel.NewHotelSvc(test.Overlays), NCACHESRV)
	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
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

func TestAuthK8s(t *testing.T) {
	// Bail out if no addr was provided.
	if K8S_ADDR == "" {
		db.DPrintf(db.ALWAYS, "No k8s addr supplied")
		return
	}
	if !TEST_AUTH {
		db.DPrintf(db.ALWAYS, "Not testing auth, skipping auth test")
		return
	}
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nil, 0)
	err1 = setupK8sState(ts)
	assert.Nil(t, err1, "Error setupK8sState: %v", err1)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	wc, err1 := hotel.NewWebClnt(ts.FsLib, ts.job)
	assert.Nil(t, err1, "Error NewWebClnt: %v", err1)
	allowedUID := 10
	s, err := hotel.RandReserveReqUser(wc, r, allowedUID)
	assert.Nil(t, err, "Error: %v", err)
	assert.Equal(t, "Reserve successfully!", s)
	disallowedUID := 20
	s, err = hotel.RandReserveReqUser(wc, r, disallowedUID)
	assert.NotNil(t, err)
	assert.NotEqual(t, "Reserve successfully!", s)
	s, err = hotel.RandLoginReq(wc, r)
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)
	ts.Shutdown()
}
