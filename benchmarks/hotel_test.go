package benchmarks_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/protdevclnt"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/hotel"
	"sigmaos/loadgen"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdev"
	rd "sigmaos/rand"
	"sigmaos/scheddclnt"
	sp "sigmaos/sigmap"
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

func MakeHotelJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, durs string, maxrpss string, fn hotelFn, justCli bool, ncache int, cachetype string, cacheNcore proc.Tcore) *HotelJobInstance {
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
	assert.Equal(ts.T, len(durslice), len(maxrpsslice), "Non-matching lengths: %v %v", durs, maxrpss)

	ji.dur = make([]time.Duration, 0, len(durslice))
	ji.maxrps = make([]int, 0, len(durslice))

	for i := range durslice {
		d, err := time.ParseDuration(durslice[i])
		assert.Nil(ts.T, err, "Bad duration %v", err)
		n, err := strconv.Atoi(maxrpsslice[i])
		assert.Nil(ts.T, err, "Bad duration %v", err)
		ji.dur = append(ji.dur, d)
		ji.maxrps = append(ji.maxrps, n)
	}

	var err error
	var svcs []hotel.Srv
	if sigmaos {
		svcs = hotel.MkHotelSvc(test.Overlays)
	}

	if ji.justCli {
		// Read job name
		sts, err := ji.GetDir("name/hotel/")
		assert.Nil(ji.T, err, "Err Get hotel dir %v", err)
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
		ji.hj, err = hotel.MakeHotelJob(ts.SigmaClnt, ji.job, svcs, N_HOTEL, cachetype, cacheNcore, nc, CACHE_GC, HOTEL_IMG_SZ_MB)
		assert.Nil(ts.T, err, "Error MakeHotelJob: %v", err)
		sdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt, ts.GetRealm())
		procs := sdc.GetRunningProcs()
		progs := make(map[string][]string)
		for sd, ps := range procs {
			progs[sd] = make([]string, 0, len(ps))
			for _, p := range ps {
				progs[sd] = append(progs[sd], p.Program)
			}
		}
		db.DPrintf(db.TEST, "Running procs:%v", progs)
		if sigmaos {
			pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{ts.SigmaClnt.FsLib}, sp.HOTELRESERVE)
			if err != nil {
				db.DFatalf("Error make reserve pdc: %v", err)
			}
			reservec = pdc
		}
	}

	if !sigmaos {
		ji.k8ssrvaddr = K8S_ADDR
		// Write a file for clients to discover the server's address.
		if !ji.justCli {
			p := hotel.JobHTTPAddrsPath(ji.job)
			mnt := sp.MkMountService(sp.MkTaddrs([]string{ji.k8ssrvaddr}))
			if err = ts.MountService(p, mnt); err != nil {
				db.DFatalf("MountService %v", err)
			}
		}
	}

	if sigmaos {
		if HOTEL_CACHE_AUTOSCALE && cachetype == "cached" && !ji.justCli {
			ji.hj.CacheAutoscaler.Run(1*time.Second, ncache)
		}
	}

	ji.wc = hotel.MakeWebClnt(ts.FsLib, ji.job)
	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.MakeLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) {
			// Run a single request.
			ji.fn(ji.wc, r)
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
		for _, s := range sp.HOTELSVC {
			stats := &protdev.SigmaRPCStats{}
			err := ji.GetFileJson(s+"/"+protdev.STATS, stats)
			assert.Nil(ji.T, err, "error get stats %v", err)
			fmt.Printf("= %s: %v\n", s, stats)
		}
		cs, err := ji.hj.StatsSrv()
		assert.Nil(ji.T, err)
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
		assert.Nil(ji.T, err, "stop %v", err)
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
	assert.Nil(ji.T, err, "Save results: %v", err)
	assert.Equal(ji.T, rep, "Done!", "Save results not ok: %v", rep)
}
