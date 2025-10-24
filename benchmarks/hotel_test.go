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

	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/hotel"
	"sigmaos/autoscale"
	"sigmaos/benchmarks"
	"sigmaos/benchmarks/loadgen"
	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
)

const (
	RAND_INIT = 12345
	HOTEL_JOB = "hotel-job"
)

type hotelFn func(wc *hotel.WebClnt, r *rand.Rand)

type HotelJobInstance struct {
	mu               sync.Mutex
	sigmaos          bool
	justCli          bool
	k8ssrvaddr       string
	cfg              *benchmarks.HotelBenchConfig
	ready            chan bool
	msc              *mschedclnt.MSchedClnt
	fn               hotelFn
	hj               *hotel.HotelJob
	lgs              []*loadgen.LoadGenerator
	p                *perf.Perf
	wc               *hotel.WebClnt
	done             bool
	cosSimAutoscaler *autoscale.Autoscaler
	// Cluster pre-warming
	warmCossimSrvKID string
	cossimKIDs       map[string]bool
	cacheKIDs        map[string]bool
	*test.RealmTstate
}

func NewHotelJob(ts *test.RealmTstate, p *perf.Perf, sigmaos bool, fn hotelFn, justCli bool, cfg *benchmarks.HotelBenchConfig) *HotelJobInstance {
	ji := &HotelJobInstance{}
	ji.sigmaos = sigmaos
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.RealmTstate = ts
	ji.p = p
	ji.justCli = justCli
	ji.cfg = cfg
	ji.cossimKIDs = make(map[string]bool)
	ji.cacheKIDs = make(map[string]bool)

	var err error

	if ji.justCli {
		// Read job name from filesystem and update the hotelCfg
		sts, err := ji.GetDir("name/hotel/")
		assert.Nil(ji.Ts.T, err, "Err Get hotel dir %v", err)
		var l int
		for _, st := range sts {
			// Dumb heuristic, but will always be the longest name....
			if len(st.Name) > l {
				ji.cfg.JobCfg.Job = st.Name
				l = len(st.Name)
			}
		}
	}

	if ji.cfg.JobCfg.UseMatch {
		ji.cfg.JobCfg.Srvs = hotel.NewHotelSvcWithMatch()
	} else {
		ji.cfg.JobCfg.Srvs = hotel.NewHotelSvc()
	}

	if !ji.justCli {
		ji.hj, err = hotel.NewHotelJob(ts.SigmaClnt, ji.cfg.GetJobConfig(), ji.cfg.CosSimBenchCfg.GetJobConfig())
		assert.Nil(ts.Ts.T, err, "Error NewHotelJob: %v", err)
		if ji.cfg.JobCfg.UseMatch {
			ji.msc = mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
			foundCossim := false
			foundCached := false
			runningProcs, err := ji.msc.GetAllRunningProcs()
			if !assert.Nil(ts.Ts.T, err, "Err GetRunningProcs: %v", err) {
				return ji
			}
			for _, p := range runningProcs[ts.GetRealm()] {
				// Record where relevant programs are running
				switch p.GetProgram() {
				case "cossim-srv-cpp":
					ji.cossimKIDs[p.GetKernelID()] = true
					db.DPrintf(db.TEST, "cossim-srv-cpp[%v] running on kernel %v", p.GetPid(), p.GetKernelID())
					foundCossim = true
				case "cached":
					ji.cacheKIDs[p.GetKernelID()] = true
					ji.warmCossimSrvKID = p.GetKernelID()
					db.DPrintf(db.TEST, "cached[%v] running on kernel %v", p.GetPid(), p.GetKernelID())
					foundCached = true
				default:
				}
			}
			if !assert.True(ts.Ts.T, foundCossim, "Err didn't find cossim srv") {
				return ji
			}
			if !assert.True(ts.Ts.T, foundCached, "Err didn't find cached srv") {
				return ji
			}
			// Warm up an msched currently running a cached shard with the cossim srv
			// bin. No cossim server will be able to actually run on this machine (the
			// CPU reservation conflicts with that of the cached server), so we can be
			// sure that future servers which try to download the cossim srver binary
			// from this msched won't have to contend with the CPU utilization of an
			// existing cossim server under load.
			db.DPrintf(db.TEST, "Target kernel to run prewarm with CossimSrv bin: %v", ji.warmCossimSrvKID)
			err = ji.msc.WarmProcd(ji.warmCossimSrvKID, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), "cossim-srv-cpp-v"+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), proc.T_LC)
			if !assert.Nil(ts.Ts.T, err, "Err warming third msched with cossim bin: %v", err) {
				return ji
			}
			db.DPrintf(db.TEST, "Warmed kid %v with CossimSrv bin", ji.warmCossimSrvKID)
		}
	}

	if !sigmaos {
		ji.k8ssrvaddr = K8S_ADDR
		// Write a file for clients to discover the server's address.
		if !ji.justCli {
			pn := hotel.JobHTTPAddrsPath(ji.cfg.JobCfg.Job)
			h, p, err := net.SplitHostPort(K8S_ADDR)
			assert.Nil(ts.Ts.T, err, "Err split host port %v: %v", ji.k8ssrvaddr, err)
			port, err := strconv.Atoi(p)
			assert.Nil(ts.Ts.T, err, "Err parse port %v: %v", p, err)
			addr := sp.NewTaddr(sp.Tip(h), sp.Tport(port))
			mnt := sp.NewEndpoint(sp.EXTERNAL_EP, []*sp.Taddr{addr})
			if err = ts.MkEndpointFile(pn, mnt); err != nil {
				db.DFatalf("MkEndpointFile mnt %v err %v", mnt, err)
			}
		}
	}

	if sigmaos {
		if ji.cfg.CacheBenchCfg.Autoscale && ji.cfg.JobCfg.Cache == "cached" && !ji.justCli {
			ji.hj.CacheAutoscaler.Run(1*time.Second, ji.cfg.JobCfg.CacheCfg.NSrv)
		}
	}

	wc, err := hotel.NewWebClnt(ts.FsLib, ji.cfg.JobCfg.Job)
	assert.Nil(ts.Ts.T, err, "Err NewWebClnt: %v", err)
	ji.wc = wc
	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.cfg.Durs))
	for i := range ji.cfg.Durs {
		ji.lgs = append(ji.lgs, loadgen.NewLoadGenerator(ji.cfg.Durs[i], ji.cfg.MaxRPS[i], func(r *rand.Rand) (time.Duration, bool) {
			// Run a single request.
			ji.fn(ji.wc, r)
			return 0, false
		}))
	}
	return ji
}

func (ji *HotelJobInstance) scaleGeoSrv() {
	// If this isn't the main benchmark driver, bail out
	if ji.justCli {
		return
	}
	if ji.cfg.ScaleGeo.GetShouldScale() {
		go func() {
			time.Sleep(ji.cfg.ScaleGeo.GetScalingDelay())
			if ji.sigmaos {
				for i := 0; i < ji.cfg.ScaleGeo.GetNToAdd(); i++ {
					err := ji.hj.AddGeoSrv()
					assert.Nil(ji.Ts.T, err, "Add Geo srv: %v", err)
				}
			} else {
				if ji.cfg.ScaleGeo.GetNToAdd() > 0 {
					err := k8sScaleUpGeo()
					assert.Nil(ji.Ts.T, err, "K8s scale up Geo srv: %v", err)
				} else {
					db.DPrintf(db.ALWAYS, "No geos meant to be added. Skip scaling up")
				}
			}
		}()
	}
}

func (ji *HotelJobInstance) scaleCaches() {
	// If this isn't the main benchmark driver, bail out
	if ji.justCli {
		return
	}
	if ji.cfg.CacheBenchCfg.ManuallyScale.GetShouldScale() {
		go func() {
			time.Sleep(ji.cfg.CacheBenchCfg.ManuallyScale.GetScalingDelay())
			ji.hj.CacheAutoscaler.AddServers(ji.cfg.CacheBenchCfg.ManuallyScale.GetNToAdd())
		}()
	}
}

func (ji *HotelJobInstance) isDone() bool {
	ji.mu.Lock()
	defer ji.mu.Unlock()
	return ji.done
}

func (ji *HotelJobInstance) scaleCosSimSrv() {
	// If this isn't the main benchmark driver, bail out
	if ji.justCli {
		return
	}
	// If not running match server, bail out
	if !ji.cfg.JobCfg.UseMatch {
		return
	}
	if ji.cfg.CosSimBenchCfg.Autoscale.GetShouldScale() {
		rifMetric := cossimsrv.NewRequestsInFlightMetric(ji.hj.CosSimJob.Clnt)
		addReplicas := func(n int) error {
			for i := 0; i < n; i++ {
				db.DPrintf(db.TEST, "Autoscaler: Scale up cossim srvs")
				err := ji.hj.AddCosSimSrvWithSigmaPath(chunk.ChunkdPath(ji.warmCossimSrvKID))
				if err != nil {
					db.DPrintf(db.TEST, "Autoscaler: Err add CosSim srv: %v", err)
					return err
				}
				db.DPrintf(db.TEST, "Autoscaler: Done scale up cossim srv")
			}
			return nil
		}
		removeReplicas := func(n int) error {
			db.DPrintf(db.TEST, "Autoscaler: Scale down not implemented, requested to remove %v replicas", n)
			return nil
		}
		ji.cosSimAutoscaler = autoscale.NewAutoscaler(
			ji.cfg.CosSimBenchCfg.Autoscale.InitialNReplicas,
			ji.cfg.CosSimBenchCfg.Autoscale.MaxReplicas,
			ji.cfg.CosSimBenchCfg.Autoscale.TargetRIF,
			ji.cfg.CosSimBenchCfg.Autoscale.Frequency,
			ji.cfg.CosSimBenchCfg.Autoscale.Tolerance,
			rifMetric,
			addReplicas,
			removeReplicas,
		)
		ji.cosSimAutoscaler.Run()
	}
	if ji.cfg.CosSimBenchCfg.ManuallyScale.GetShouldScale() {
		go func() {
			time.Sleep(ji.cfg.CosSimBenchCfg.ManuallyScale.GetScalingDelay())
			for i := 0; i < ji.cfg.CosSimBenchCfg.ManuallyScale.GetNToAdd(); i++ {
				db.DPrintf(db.TEST, "Scale up cossim srvs to: %v", (i+1)+ji.cfg.CosSimBenchCfg.JobCfg.InitNSrv)
				err := ji.hj.AddCosSimSrvWithSigmaPath(chunk.ChunkdPath(ji.warmCossimSrvKID))
				assert.Nil(ji.Ts.T, err, "Add CosSim srv: %v", err)
				db.DPrintf(db.TEST, "Done scale up cossim srvs to: %v", (i+1)+ji.cfg.CosSimBenchCfg.JobCfg.InitNSrv)
			}
		}()
	}
}

func (ji *HotelJobInstance) StartHotelJob() {
	db.DPrintf(db.ALWAYS, "StartHotelJob kubernetes (%v,%v) cfg:%v", !ji.sigmaos, ji.k8ssrvaddr, ji.cfg)
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
	go ji.scaleGeoSrv()
	go ji.scaleCaches()
	go ji.scaleCosSimSrv()
	for i, lg := range ji.lgs {
		db.DPrintf(db.TEST, "Run load generator rps %v dur %v", ji.cfg.MaxRPS[i], ji.cfg.Durs[i])
		lg.Run()
		//    ji.printStats()
	}
	db.DPrintf(db.ALWAYS, "Done running HotelJob")
}

func (ji *HotelJobInstance) printStats() {
	if ji.sigmaos && !ji.justCli {
		for _, s := range hotel.HOTELSVC {
			// Hotel geo doesn't register itself in the FS anymore
			if strings.Contains(s, hotel.HOTELGEODIR) {
				continue
			}
			stats, err := ji.ReadSrvStats(s)
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
	time.Sleep(20 * time.Second)
	if ji.p != nil {
		ji.p.Done()
	}
	ji.mu.Lock()
	ji.done = true
	ji.mu.Unlock()
	if ji.cosSimAutoscaler != nil {
		ji.cosSimAutoscaler.Stop()
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
