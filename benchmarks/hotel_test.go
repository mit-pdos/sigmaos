package benchmarks_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/hotel"
	"sigmaos/loadgen"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	rd "sigmaos/rand"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	RAND_INIT = 12345
)

type hotelFn func(wc *hotel.WebClnt, r *rand.Rand)

type HotelJobInstance struct {
	sigmaos    bool
	k8ssrvaddr string
	job        string
	dur        []time.Duration
	maxrps     []int
	ready      chan bool
	fn         hotelFn
	pids       []proc.Tpid
	cc         *cacheclnt.CacheClnt
	cm         *cacheclnt.CacheMgr
	lgs        []*loadgen.LoadGenerator
	*test.Tstate
}

func MakeHotelJob(ts *test.Tstate, sigmaos bool, durs string, maxrpss string, fn hotelFn) *HotelJobInstance {
	ji := &HotelJobInstance{}
	ji.sigmaos = sigmaos
	ji.job = rd.String(8)
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.Tstate = ts

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
	var ncache int
	var svcs []string
	if sigmaos {
		svcs = hotel.HotelSvcs
		ncache = hotel.NCACHE
	}
	ji.cc, ji.cm, ji.pids, err = hotel.MakeHotelJob(ts.FsLib, ts.ProcClnt, ji.job, svcs, ncache)
	assert.Nil(ts.T, err, "Error MakeHotelJob: %v", err)

	if !sigmaos {
		ji.k8ssrvaddr = K8S_ADDR
		// Write a file for clients to discover the server's address.
		p := hotel.JobHTTPAddrsPath(ji.job)
		if err := ts.PutFileJson(p, 0777, []string{ji.k8ssrvaddr}); err != nil {
			db.DFatalf("Error PutFileJson addrs %v", err)
		}
	}

	wc := hotel.MakeWebClnt(ts.FsLib, ji.job)
	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.MakeLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) {
			// Run a single request.
			ji.fn(wc, r)
		}))
	}
	return ji
}

func (ji *HotelJobInstance) StartHotelJob() {
	db.DPrintf(db.ALWAYS, "StartHotelJob dur %v maxrps %v kubernetes (%v,%v)", ji.dur, ji.maxrps, !ji.sigmaos, ji.k8ssrvaddr)
	var wg sync.WaitGroup
	for _, lg := range ji.lgs {
		wg.Add(1)
		go func(lg *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			lg.Calibrate()
		}(lg, &wg)
	}
	wg.Wait()
	for i, lg := range ji.lgs {
		db.DPrintf("TEST", "Run load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
		lg.Run()
	}
	db.DPrintf(db.ALWAYS, "Done running HotelJob")
}

func (ji *HotelJobInstance) printStats() {
	if ji.sigmaos {
		for _, s := range sp.HOTELSVC {
			stats := &protdevsrv.Stats{}
			err := ji.GetFileJson(s+"/"+protdevsrv.STATS, stats)
			assert.Nil(ji.T, err, "error get stats %v", err)
			fmt.Printf("= %s: %v\n", s, stats)
		}
		cs, err := ji.cc.StatsSrv()
		assert.Nil(ji.T, err)
		for i, cstat := range cs {
			fmt.Printf("= cache-%v: %v\n", i, cstat)
		}
	}
}

func (ji *HotelJobInstance) Wait() {
	db.DPrintf("TEST", "Evicting hotel procs")
	ji.printStats()
	if ji.sigmaos {
		for _, pid := range ji.pids {
			err := ji.Evict(pid)
			assert.Nil(ji.T, err, "Evict: %v", err)
			_, err = ji.WaitExit(pid)
			assert.Nil(ji.T, err)
		}
		ji.cm.StopCache()
	}
	for _, lg := range ji.lgs {
		db.DPrintf(db.ALWAYS, "Data:\n%v", lg.StatsDataString())
	}
	for _, lg := range ji.lgs {
		lg.Stats()
	}
}

func (ji *HotelJobInstance) requestK8sStats() {
	wc := hotel.MakeWebClnt(ji.FsLib, ji.job)
	rep, err := wc.SaveResults()
	assert.Nil(ji.T, err, "Save results: %v", err)
	assert.Equal(ji.T, rep, "Done!", "Save results not ok: %v", rep)
}
