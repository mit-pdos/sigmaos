package benchmarks_test

import (
	"math/rand"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/hotel"
	"sigmaos/loadgen"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/test"
)

type HotelJobInstance struct {
	sigmaos    bool
	k8ssrvaddr string
	ncore      proc.Tcore // Number of exclusive cores allocated to each server
	job        string
	dur        time.Duration
	maxrps     int
	ready      chan bool
	pids       []proc.Tpid
	cc         *cacheclnt.CacheClnt
	cm         *cacheclnt.CacheMgr
	lg         *loadgen.LoadGenerator
	*test.Tstate
}

func MakeHotelJob(ts *test.Tstate, sigmaos bool, ncore proc.Tcore, dur time.Duration, maxrps int) *HotelJobInstance {
	ji := &HotelJobInstance{}
	ji.sigmaos = sigmaos
	ji.ncore = ncore
	ji.job = rd.String(8)
	ji.dur = dur
	ji.maxrps = maxrps
	ji.ready = make(chan bool)
	ji.Tstate = ts

	var err error
	ji.cc, ji.cm, ji.pids, err = hotel.MakeHotelJob(ts.FsLib, ts.ProcClnt, ji.job, hotel.HotelSvcs, ncore, hotel.NCACHE)
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Make a load generator.
	ji.lg = loadgen.MakeLoadGenerator(ji.dur, ji.maxrps, func() {
		// Run a single DSB hotel request.
		hotel.RunDSB(ts.T, 1, wc, r)
	})

	return ji
}

func (ji *HotelJobInstance) StartHotelJob() {
	db.DPrintf(db.ALWAYS, "StartHotelJob dur %v kubernetes (%v,%v) maxrps %v", ji.dur, !ji.sigmaos, ji.k8ssrvaddr, ji.maxrps)
	ji.lg.Run()
}

func (ji *HotelJobInstance) Wait() {
	if ji.sigmaos {
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
