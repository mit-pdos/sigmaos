package benchmarks_test

import (
	"fmt"
	"math/rand"
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
	ncore      proc.Tcore // Number of exclusive cores allocated to each server
	job        string
	dur        time.Duration
	maxrps     int
	ready      chan bool
	fn         hotelFn
	pids       []proc.Tpid
	cc         *cacheclnt.CacheClnt
	cm         *cacheclnt.CacheMgr
	lg         *loadgen.LoadGenerator
	*test.Tstate
}

func MakeHotelJob(ts *test.Tstate, sigmaos bool, ncore proc.Tcore, dur time.Duration, maxrps int, fn hotelFn) *HotelJobInstance {
	ji := &HotelJobInstance{}
	ji.sigmaos = sigmaos
	ji.ncore = ncore
	ji.job = rd.String(8)
	ji.dur = dur
	ji.maxrps = maxrps
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.Tstate = ts

	var err error
	var ncache int
	var svcs []string
	if sigmaos {
		svcs = hotel.HotelSvcs
		ncache = hotel.NCACHE
	}
	ji.cc, ji.cm, ji.pids, err = hotel.MakeHotelJob(ts.FsLib, ts.ProcClnt, ji.job, svcs, ncore, ncache)
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
	r := rand.New(rand.NewSource(12345))
	// Make a load generator.
	ji.lg = loadgen.MakeLoadGenerator(ji.dur, ji.maxrps, func() {
		// Run a single request.
		ji.fn(wc, r)
	})

	return ji
}

func (ji *HotelJobInstance) StartHotelJob() {
	db.DPrintf(db.ALWAYS, "StartHotelJob dur %v maxrps %v kubernetes (%v,%v)", ji.dur, ji.maxrps, !ji.sigmaos, ji.k8ssrvaddr)
	ji.lg.Run()
	db.DPrintf(db.ALWAYS, "Done running HotelJob")
}

func (ji *HotelJobInstance) PrintStats(lg *loadgen.LoadGenerator) {
	if lg != nil {
		lg.Stats()
	}
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

func (ji *HotelJobInstance) Wait() {
	db.DPrintf("TEST", "Evicting hotel procs")
	if ji.sigmaos {
		ji.PrintStats(nil)
		for _, pid := range ji.pids {
			err := ji.Evict(pid)
			assert.Nil(ji.T, err, "Evict: %v", err)
			_, err = ji.WaitExit(pid)
			assert.Nil(ji.T, err)
		}
		ji.cm.StopCache()
	}
}

// XXX print stats
//	for _, s := range np.HOTELSVC {
//		ts.statsSrv(s)
//	}
//	cs, err := ts.cc.StatsSrv()
//	assert.Nil(ts.T, err)
//	for i, cstat := range cs {
//		fmt.Printf("= cache-%v: %v\n", i, cstat)
//	}
