package benchmarks_test

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/rpcclnt"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/hotel"
	"sigmaos/loadgen"
	"sigmaos/perf"
	"sigmaos/proc"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
	"sigmaos/test"
)

const (
	RAND_INIT = 12345
)

type hotelFn func(wc *hotel.WebClnt, r *rand.Rand)

type HotelJobInstance struct {
	sigmaos    bool
	justCli    bool
	k8ssrvaddr string
	job        string
	dur        []time.Duration
	maxrps     []int
	ncache     int
	cachetype  string
	ready      chan bool
	fn         hotelFn
	hj         *hotel.HotelJob
	lgs        []*loadgen.LoadGenerator
	p          *perf.Perf
	wc         *hotel.WebClnt
	*test.RealmTstate
}

func NewHotelJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, durs string, maxrpss string, fn hotelFn, justCli bool, ncache int, cachetype string, cacheMcpu proc.Tmcpu) *HotelJobInstance {
	ji := &HotelJobInstance{}
	ji.sigmaos = sigmaos
	ji.job = rd.String(8)
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.RealmTstate = ts
	ji.p = p
	ji.justCli = justCli
	ji.ncache = ncache
	ji.cachetype = cachetype

	durslice := strings.Split(durs, ",")
	maxrpsslice := strings.Split(maxrpss, ",")
	assert.Equal(ts.Ts.T, len(durslice), len(maxrpsslice), "Non-matching lengths: %v %v", durs, maxrpss)

	ji.dur = make([]time.Duration, 0, len(durslice))
	ji.maxrps = make([]int, 0, len(durslice))

	for i := range durslice {
		d, err := time.ParseDuration(durslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		n, err := strconv.Atoi(maxrpsslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		ji.dur = append(ji.dur, d)
		ji.maxrps = append(ji.maxrps, n)
	}

	var err error
	var svcs []hotel.Srv
	if sigmaos {
		svcs = hotel.NewHotelSvc(test.Overlays)
	}

	if ji.justCli {
		// Read job name
		sts, err := ji.GetDir("name/hotel/")
		assert.Nil(ji.Ts.T, err, "Err Get hotel dir %v", err)
		var l int
		for _, st := range sts {
			// Dumb heuristic, but will always be the longest name....
			if len(st.Name) > l {
				ji.job = st.Name
				l = len(st.Name)
			}
		}
	}

	if !ji.justCli {
		if sigmaos {
			if CACHE_TYPE == "memcached" {
				addrs := strings.Split(MEMCACHED_ADDRS, ",")
				err := ts.SigmaClnt.PutFileJson(sp.MEMCACHED, 0777, addrs)
				if err != nil {
					db.DFatalf("Error put memcached file")
				}
			}
		}
		var nc = ncache
		// Only start one cache if autoscaling.
		if sigmaos && CACHE_TYPE == "cached" && HOTEL_CACHE_AUTOSCALE {
			nc = 1
		}
		ji.hj, err = hotel.NewHotelJob(ts.SigmaClnt, ji.job, svcs, N_HOTEL, cachetype, cacheMcpu, nc, CACHE_GC, HOTEL_IMG_SZ_MB)
		assert.Nil(ts.Ts.T, err, "Error NewHotelJob: %v", err)
		if sigmaos {
			ch, err := sigmarpcchan.NewSigmaRPCCh([]*fslib.FsLib{ts.SigmaClnt.FsLib}, hotel.HOTELRESERVE)
			if err != nil {
				db.DFatalf("Error make reserve pdc: %v", err)
			}
			rpcc := rpcclnt.NewRPCClnt(ch)
			reservec = rpcc
		}
	}

	if !sigmaos {
		ji.k8ssrvaddr = K8S_ADDR
		// Write a file for clients to discover the server's address.
		if !ji.justCli {
			p := hotel.JobHTTPAddrsPath(ji.job)
			h, p, err := net.SplitHostPort(K8S_ADDR)
			assert.Nil(ts.Ts.T, err, "Err split host port %v: %v", ji.k8ssrvaddr, err)
			port, err := strconv.Atoi(p)
			assert.Nil(ts.Ts.T, err, "Err parse port %v: %v", p, err)
			addr := sp.NewTaddrRealm(sp.Tip(h), sp.INNER_CONTAINER_IP, sp.Tport(port), ts.ProcEnv().GetNet())
			mnt := sp.NewEndpoint([]*sp.Taddr{addr}, ts.ProcEnv().GetRealm())
			if err = ts.MkEndpointFile(p, mnt, sp.NoLeaseId); err != nil {
				db.DFatalf("MkEndpointFile %v", err)
			}
		}
	}

	if sigmaos {
		if HOTEL_CACHE_AUTOSCALE && cachetype == "cached" && !ji.justCli {
			ji.hj.CacheAutoscaler.Run(1*time.Second, ncache)
		}
	}

	wc, err := hotel.NewWebClnt(ts.FsLib, ji.job)
	assert.Nil(ts.Ts.T, err, "Err NewWebClnt: %v", err)
	ji.wc = wc
	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.NewLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) (time.Duration, bool) {
			// Run a single request.
			ji.fn(ji.wc, r)
			return 0, false
		}))
	}
	return ji
}

func (ji *HotelJobInstance) StartHotelJob() {
	db.DPrintf(db.ALWAYS, "StartHotelJob dur %v ncache %v maxrps %v kubernetes (%v,%v)", ji.dur, ji.ncache, ji.maxrps, !ji.sigmaos, ji.k8ssrvaddr)
	var wg sync.WaitGroup
	for _, lg := range ji.lgs {
		wg.Add(1)
		go func(lg *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			lg.Calibrate()
		}(lg, &wg)
	}
	wg.Wait()
	_, err := ji.wc.StartRecording()
	if err != nil {
		db.DFatalf("Can't start recording: %v", err)
	}
	for i, lg := range ji.lgs {
		db.DPrintf(db.TEST, "Run load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
		lg.Run()
		//    ji.printStats()
	}
	db.DPrintf(db.ALWAYS, "Done running HotelJob")
}

func (ji *HotelJobInstance) printStats() {
	if ji.sigmaos && !ji.justCli {
		for _, s := range hotel.HOTELSVC {
			stats, err := ji.ReadStats(s)
			assert.Nil(ji.Ts.T, err, "error get stats [%v] %v", s, err)
			fmt.Printf("= %s: %v\n", s, stats)
		}
		cs, err := ji.hj.StatsSrv()
		assert.Nil(ji.Ts.T, err)
		for i, cstat := range cs {
			fmt.Printf("= cache-%v: %v\n", i, cstat)
		}
	}
}

func (ji *HotelJobInstance) Wait() {
	db.DPrintf(db.TEST, "extra sleep")
	time.Sleep(10 * time.Second)
	if ji.p != nil {
		ji.p.Done()
	}
	db.DPrintf(db.TEST, "Evicting hotel procs")
	if ji.sigmaos && !ji.justCli {
		ji.printStats()
		err := ji.hj.Stop()
		assert.Nil(ji.Ts.T, err, "stop %v", err)
	}
	db.DPrintf(db.TEST, "Done evicting hotel procs")
	for _, lg := range ji.lgs {
		db.DPrintf(db.ALWAYS, "Data:\n%v", lg.StatsDataString())
	}
	for _, lg := range ji.lgs {
		lg.Stats()
	}
}

func (ji *HotelJobInstance) requestK8sStats() {
	rep, err := ji.wc.SaveResults()
	assert.Nil(ji.Ts.T, err, "Save results: %v", err)
	assert.Equal(ji.Ts.T, rep, "Done!", "Save results not ok: %v", rep)
}
