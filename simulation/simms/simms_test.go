package simms_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/autoscaler"
	"sigmaos/simulation/simms/opts"
	"sigmaos/simulation/simms/qmgr"
)

func TestCompile(t *testing.T) {
}

func TestClients(t *testing.T) {
	const (
		N_TICKS       = 1000
		CLNT_REQ_MEAN = 1
		CLNT_REQ_STD  = 0
	)
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	for i := uint64(0); i < N_TICKS; i++ {
		reqs := c.Tick(i)
		assert.Equal(t, 1, len(reqs), "Wrong num requests")
		assert.Equal(t, reqs[0].GetStart(), i)
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestServiceInstanceNoQueueBuildup(t *testing.T) {
	const (
		N_TICKS        uint64 = 1000
		N_SLOTS        int    = 1
		P_TIME         uint64 = 1
		N_REQ_PER_TICK int    = 1
		SVC_ID         string = "svc"
		STATEFUL       bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewServiceInstance(&time, p, 0, qmgr.NewBasicQMgr(&time, nil))
	for ; time < N_TICKS; time++ {
		// Construct requests
		reqs := make([]*simms.Request, N_REQ_PER_TICK)
		for i := range reqs {
			reqs[i] = simms.NewRequest(time)
		}
		// Process requests
		reps := svc.Tick(reqs)
		if time < P_TIME {
			// Should get no replies on the first few ticks (nothing has processed
			// yet)
			assert.Equal(t, 0, len(reps), "Processed some requests on first tick")
		} else {
			assert.Equal(t, N_REQ_PER_TICK, len(reps), "Produced wrong number of replies")
			for _, rep := range reps {
				assert.Equal(t, P_TIME, rep.GetLatency(), "Wrong latency")
			}
		}
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestServiceInstanceNoQueueBuildup10ReqPerTick(t *testing.T) {
	const (
		N_TICKS        uint64 = 1000
		N_SLOTS        int    = 10
		P_TIME         uint64 = 1
		N_REQ_PER_TICK int    = 10
		SVC_ID         string = "svc"
		STATEFUL       bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewServiceInstance(&time, p, 0, qmgr.NewBasicQMgr(&time, nil))
	for ; time < N_TICKS; time++ {
		// Construct requests
		reqs := make([]*simms.Request, N_REQ_PER_TICK)
		for i := range reqs {
			reqs[i] = simms.NewRequest(time)
		}
		// Process requests
		reps := svc.Tick(reqs)
		if time < P_TIME {
			// Should get no replies on the first few ticks (nothing has processed
			// yet)
			assert.Equal(t, 0, len(reps), "Processed some requests on first tick")
		} else {
			assert.Equal(t, N_REQ_PER_TICK, len(reps), "Produced wrong number of replies")
			for _, rep := range reps {
				assert.Equal(t, P_TIME, rep.GetLatency(), "Wrong latency")
			}
		}
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestServiceInstanceQueueBuildup10ReqPerTick(t *testing.T) {
	const (
		N_TICKS        uint64 = 10
		N_SLOTS        int    = 1
		P_TIME         uint64 = 1
		N_REQ_PER_TICK int    = 2
		SVC_ID         string = "svc"
		STATEFUL       bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewServiceInstance(&time, p, 0, qmgr.NewBasicQMgr(&time, nil))
	for ; time < N_TICKS; time++ {
		// Construct requests
		reqs := make([]*simms.Request, N_REQ_PER_TICK)
		for i := range reqs {
			reqs[i] = simms.NewRequest(time)
		}
		// Process requests
		reps := svc.Tick(reqs)
		if time < P_TIME {
			// Should get no replies on the first few ticks (nothing has processed
			// yet)
			assert.Equal(t, 0, len(reps), "Processed some requests on first tick")
		} else {
			assert.Equal(t, N_SLOTS, len(reps), "Produced wrong number of replies")
			for _, rep := range reps {
				assert.Equal(t, time/uint64(N_REQ_PER_TICK)+P_TIME, rep.GetLatency(), "Wrong latency")
			}
		}
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestServiceInstanceNoQueueBuildupPTime2(t *testing.T) {
	const (
		N_TICKS        uint64 = 1000
		N_SLOTS        int    = 4
		P_TIME         uint64 = 2
		N_REQ_PER_TICK int    = 2
		SVC_ID         string = "svc"
		STATEFUL       bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewServiceInstance(&time, p, 0, qmgr.NewBasicQMgr(&time, nil))
	for ; time < N_TICKS; time++ {
		// Construct requests
		reqs := make([]*simms.Request, N_REQ_PER_TICK)
		for i := range reqs {
			reqs[i] = simms.NewRequest(time)
		}
		// Process requests
		reps := svc.Tick(reqs)
		if time < P_TIME {
			// Should get no replies on the first few ticks (nothing has processed
			// yet)
			assert.Equal(t, 0, len(reps), "Processed some requests on first tick")
		} else {
			assert.Equal(t, N_REQ_PER_TICK, len(reps), "Produced wrong number of replies")
			for _, rep := range reps {
				assert.Equal(t, P_TIME, rep.GetLatency(), "Wrong latency")
			}
		}
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestServiceInstanceQueueBuildupPTime2(t *testing.T) {
	const (
		N_TICKS        uint64 = 10
		N_SLOTS        int    = 1
		P_TIME         uint64 = 2
		N_REQ_PER_TICK int    = 1
		SVC_ID         string = "svc"
		STATEFUL       bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewServiceInstance(&time, p, 0, qmgr.NewBasicQMgr(&time, nil))
	for ; time < N_TICKS; time++ {
		// Construct requests
		reqs := make([]*simms.Request, N_REQ_PER_TICK)
		for i := range reqs {
			reqs[i] = simms.NewRequest(time)
		}
		// Process requests
		reps := svc.Tick(reqs)
		if time < P_TIME || time%P_TIME > 0 {
			// Should get no replies on the first few ticks (nothing has processed
			// yet)
			assert.Equal(t, 0, len(reps), "Processed some requests on first tick")
		} else {
			assert.Equal(t, N_SLOTS, len(reps), "[t=%v] Produced wrong number of replies", time)
			for _, rep := range reps {
				// Queue length L at time T is (T + 1) / P_TIME
				// Time to consume a queue of length L is (L + 1) * P_TIME
				// Number of requests processed at time T is T / P_TIME
				// At time T, the processed request is the (T / P_TIME) - 1 -th request, which had to wait (T - 1) / P_TIME before it began to be processed
				assert.Equal(t, (time*uint64(N_REQ_PER_TICK)-1)/P_TIME+P_TIME, rep.GetLatency(), "[t=%v] Wrong latency", time)
			}
		}
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAppNoQueueBuildup(t *testing.T) {
	const (
		N_TICKS        uint64 = 1000
		N_SLOTS        int    = 1
		P_TIME         uint64 = 1
		N_REQ_PER_TICK int    = 1
		SVC_ID         string = "wfe"
		STATEFUL       bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts)
	app := simms.NewSingleTierApp(svc)
	for ; time < N_TICKS; time++ {
		// Construct requests
		reqs := make([]*simms.Request, N_REQ_PER_TICK)
		for i := range reqs {
			reqs[i] = simms.NewRequest(time)
		}
		// Process requests
		reps := app.Tick(reqs)
		if time < P_TIME {
			// Should get no replies on the first few ticks (nothing has processed
			// yet)
			assert.Equal(t, 0, len(reps), "Processed some requests on first tick")
		} else {
			assert.Equal(t, N_REQ_PER_TICK, len(reps), "Produced wrong number of replies")
			for _, rep := range reps {
				assert.Equal(t, P_TIME, rep.GetLatency(), "Wrong latency")
			}
		}
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestWorkloadNoQueueBuildup(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN float64 = 1
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS  int    = 1
		P_TIME   uint64 = 1
		SVC_ID   string = "wfe"
		STATEFUL bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts)
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	for ; time < N_TICKS; time++ {
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	assert.Equal(t, int(N_TICKS)-1, stats.TotalRequests(), "Produced wrong number of replies")
	assert.Equal(t, float64(P_TIME), stats.AvgLatency(), "Produced wrong number of replies")
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestWorkloadClntBurst(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 1
		CLNT_REQ_STD     float64 = 0
		BURST_START      uint64  = 500
		BURST_END        uint64  = 1000
		BURST_MULTIPLIER float64 = 2.0
		// App params
		N_SLOTS  int    = 1
		P_TIME   uint64 = 1
		SVC_ID   string = "wfe"
		STATEFUL bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts)
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		if time == BURST_END {
			c.EndBurst()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestWorkloadClntBurstAddInstance(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 1
		CLNT_REQ_STD     float64 = 0
		BURST_START      uint64  = 500
		BURST_END        uint64  = 1000
		BURST_MULTIPLIER float64 = 2.0
		// App params
		N_SLOTS  int    = 1
		P_TIME   uint64 = 1
		SVC_ID   string = "wfe"
		STATEFUL bool   = false
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts)
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
			svc.AddInstance()
		}
		if time == BURST_END {
			c.EndBurst()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	assert.Equal(t, float64(1.0), stats.AvgLatency())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestWorkloadClntBurstRemoveInstance(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 1
		CLNT_REQ_STD     float64 = 0
		BURST_START      uint64  = 0
		BURST_END        uint64  = 1000
		BURST_MULTIPLIER float64 = 2.0
		// App params
		N_SLOTS             int    = 1
		P_TIME              uint64 = 1
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		SIZE_UP_TIME        uint64 = 0
		SIZE_DOWN_TIME      uint64 = 500
		RECORD_STATS_WINDOW int    = 10
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts)
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			db.DPrintf(db.SIM_TEST, "StartBurst [t=%v]", time)
			c.StartBurst(BURST_MULTIPLIER)
		}
		if time == BURST_END {
			db.DPrintf(db.SIM_TEST, "EndBurst [t=%v]", time)
			c.EndBurst()
		}
		if time == SIZE_UP_TIME {
			svc.AddInstance()
		}
		if time == SIZE_DOWN_TIME {
			svc.RemoveInstance()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtilAutoscaler(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN float64 = 1
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 1
		P_TIME              uint64 = 1
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		SIZE_UP_TIME        uint64 = 0
		SIZE_DOWN_TIME      uint64 = 500
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = autoscaler.UNLIMITED_REPLICAS
		SCALE_FREQ         int     = 10
		TARGET_UTIL        float64 = 0.5
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 100 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp))
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	assert.Equal(t, 2, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	assert.Equal(t, 1, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtilAutoscalerPersistentQueueImbalance(t *testing.T) {
	const (
		N_TICKS uint64 = 5000
		// Clnt params
		CLNT_REQ_MEAN float64 = 45
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 2
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		SIZE_UP_TIME        uint64 = 0
		SIZE_DOWN_TIME      uint64 = 500
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = autoscaler.UNLIMITED_REPLICAS
		SCALE_FREQ         int     = 1
		TARGET_UTIL        float64 = 0.9
		UTIL_WINDOW_SIZE   uint64  = 1
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp))
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	assert.Equal(t, 9, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	assert.Equal(t, 0, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtilAutoscalerOscillation(t *testing.T) {
	const (
		N_TICKS uint64 = 600
		// Clnt params
		CLNT_REQ_MEAN float64 = 45
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 2
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		SIZE_UP_TIME        uint64 = 0
		SIZE_DOWN_TIME      uint64 = 500
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = autoscaler.UNLIMITED_REPLICAS
		SCALE_FREQ         int     = 1
		TARGET_UTIL        float64 = 0.5
		UTIL_WINDOW_SIZE   uint64  = 1
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp))
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtilAutoscalerResolveQueueImbalanceWithOmniscientQLenLB(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN float64 = 45
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 2
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		SIZE_UP_TIME        uint64 = 0
		SIZE_DOWN_TIME      uint64 = 500
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = autoscaler.UNLIMITED_REPLICAS
		SCALE_FREQ         int     = 1
		TARGET_UTIL        float64 = 0.9
		UTIL_WINDOW_SIZE   uint64  = 1
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp), opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	// Check that queues balanced out eventually, and request latencies settled down again
	for i := N_TICKS * 9 / 10; i < N_TICKS/10; i++ {
		assert.Equal(t, P_TIME, rstats.P99Latency[i], "Latency didn't settle to processing time")
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtilAutoscalerResolveQueueImbalanceWithNRandomQLenLB(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN float64 = 135
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 2
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		SIZE_UP_TIME        uint64 = 0
		SIZE_DOWN_TIME      uint64 = 500
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = autoscaler.UNLIMITED_REPLICAS
		SCALE_FREQ         int     = 1
		TARGET_UTIL        float64 = 0.9
		UTIL_WINDOW_SIZE   uint64  = 1
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
		// LB param
		N_RANDOM_CHOICES int = 3 // Going from 2 -> 3 decreases tail latency by ~50%
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp), opts.WithNRandomChoicesLB(N_RANDOM_CHOICES), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	// Check that queues balanced out eventually, and request latencies settled down again
	for i := N_TICKS * 9 / 10; i < N_TICKS/10; i++ {
		assert.Equal(t, P_TIME, rstats.P99Latency[i], "Latency didn't settle to processing time")
	}
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Test increase in tail latency when scaling up a service, assuming sclaing
// begins exactly when the request burst begins
func TestImmediateScaleUpWithClientBurstOmniscientQLenLB(t *testing.T) {
	const (
		N_TICKS uint64 = 250
		// Clnt params
		CLNT_REQ_MEAN         float64 = 8
		CLNT_REQ_STD          float64 = 0
		CLNT_BURST_START      uint64  = 100
		CLNT_BURST_MULTIPLIER float64 = 2.0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 1
		SVC_ID              string = "wfe"
		SVC_INIT_TIME       uint64 = 4
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, SVC_INIT_TIME, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == CLNT_BURST_START {
			// Start a burst of client requests
			c.StartBurst(CLNT_BURST_MULTIPLIER)
			// With no delay, start scaling up the service
			svc.AddInstance()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Test increase in tail latency when scaling up a service, assuming sclaing
// begins exactly when the request burst begins
func TestImmediateScaleUpWithClientBurstNRandomChoicesQLenLB(t *testing.T) {
	const (
		N_TICKS uint64 = 250
		// Clnt params
		CLNT_REQ_MEAN         float64 = 80
		CLNT_REQ_STD          float64 = 0
		CLNT_BURST_START      uint64  = 100
		CLNT_BURST_MULTIPLIER float64 = 2.0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 1
		SVC_ID              string = "wfe"
		SVC_INIT_TIME       uint64 = 4
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		N_INSTANCES         int    = 10
		// LB param
		N_RANDOM_CHOICES int = 3
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, SVC_INIT_TIME, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithNRandomChoicesLB(N_RANDOM_CHOICES), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	// Add some initial instances for the load balancer to have more choices
	for i := 1; i < N_INSTANCES; i++ {
		svc.AddInstance()
		svc.MarkInstanceReady(i)
	}
	for ; time < N_TICKS; time++ {
		if time == CLNT_BURST_START {
			// Start a burst of client requests
			c.StartBurst(CLNT_BURST_MULTIPLIER)
			// With no delay, scaling up the service by 2x
			for i := 0; i < N_INSTANCES; i++ {
				svc.AddInstance()
			}
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Test increase in tail latency when scaling up a service, assuming sclaing
// begins exactly when the request burst begins
func TestDelayedScaleUpWithClientBurstNRandomChoicesLB(t *testing.T) {
	const (
		N_TICKS uint64 = 250
		// Clnt params
		CLNT_REQ_MEAN         float64 = 80
		CLNT_REQ_STD          float64 = 0
		CLNT_BURST_START      uint64  = 100
		CLNT_BURST_MULTIPLIER float64 = 2.0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 1
		SVC_ID              string = "wfe"
		SVC_INIT_TIME       uint64 = 4
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		N_INSTANCES         int    = 10
		// Scale up params
		SCALE_UP_DELAY uint64 = 20
		// LB param
		N_RANDOM_CHOICES int = 3
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, SVC_INIT_TIME, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithNRandomChoicesLB(N_RANDOM_CHOICES), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	// Add some initial instances for the load balancer to have more choices
	for i := 1; i < N_INSTANCES; i++ {
		svc.AddInstance()
		svc.MarkInstanceReady(i)
	}
	for ; time < N_TICKS; time++ {
		if time == CLNT_BURST_START {
			// Start a burst of client requests
			c.StartBurst(CLNT_BURST_MULTIPLIER)
		}
		// Scale up instances after a (configurable) delay after the client burst starts
		if time == CLNT_BURST_START+SCALE_UP_DELAY {
			for i := 0; i < N_INSTANCES; i++ {
				svc.AddInstance()
			}
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtil50AutoscalerRRLBMatchWithK8s(t *testing.T) {
	const (
		N_TICKS uint64 = 180
		// Clnt params
		CLNT_REQ_MEAN float64 = 23
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 1
		INIT_TIME           uint64 = 5
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = 5
		SCALE_FREQ         int     = 20
		TARGET_UTIL        float64 = 0.5
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp))
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtil90AutoscalerRRLBMatchWithK8s(t *testing.T) {
	const (
		N_TICKS uint64 = 600
		// Clnt params
		CLNT_REQ_MEAN float64 = 45
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 1
		INIT_TIME           uint64 = 5
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = 5
		SCALE_FREQ         int     = 20
		TARGET_UTIL        float64 = 0.9
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp))
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestAvgUtil90AutoscalerOmniscientQLenLBMatchWithK8s(t *testing.T) {
	const (
		N_TICKS uint64 = 600
		// Clnt params
		CLNT_REQ_MEAN float64 = 45
		CLNT_REQ_STD  float64 = 0
		// App params
		N_SLOTS             int    = 10
		P_TIME              uint64 = 1
		INIT_TIME           uint64 = 5
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = 5
		SCALE_FREQ         int     = 20
		TARGET_UTIL        float64 = 0.9
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp), opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 5x load burst, with an avg util autoscaler with target
// utilization of 90%, an omniscient qlen-based load balancer, and SigmaOS
// scaling parameters (assuming only cold-starts). 1t = 1ms
func TestBurst5xK8sAvgUtilAutoscalerOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 20000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 5.0 // Burst to 5x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 8  // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = 5    // At most 5 instances should be needed to handle max load at target utilization
		SCALE_FREQ         int     = 1000 // Make scaling decisions 1/sec
		TARGET_UTIL        float64 = 0.9  // Target utilization of 90%
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp), opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 2x load burst, with an avg util autoscaler with target
// utilization of 90%, an omniscient qlen-based load balancer, and SigmaOS
// scaling parameters (assuming only cold-starts). 1t = 1ms
func TestBurst2xK8sAvgUtilAutoscalerOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 5000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 2.0 // Burst to 2x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 8  // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = 2    // At most 2 instances should be needed to handle max load at target utilization
		SCALE_FREQ         int     = 1000 // Make scaling decisions 1/sec
		TARGET_UTIL        float64 = 0.9  // Target utilization of 90%
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp), opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 5x load burst, with an avg util autoscaler with target
// utilization of 90%, an omniscient qlen-based load balancer, and SigmaOS
// scaling parameters (assuming only cold-starts). Allow scaling 2x beyond
// final capacity. 1t = 1ms
func TestBurst5xOverscale2xK8sAvgUtilAutoscalerOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 8000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 5.0 // Burst to 5x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 8  // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = 10   // At most 5 instances should be needed to handle max load at target utilization
		SCALE_FREQ         int     = 1000 // Make scaling decisions 1/sec
		TARGET_UTIL        float64 = 0.9  // Target utilization of 90%
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp), opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 2x load burst, with an avg util autoscaler with target
// utilization of 90%, an omniscient qlen-based load balancer, and SigmaOS
// scaling parameters (assuming only cold-starts). Allow scaling 2x beyond
// final capacity. 1t = 1ms
func TestBurst2xOverscale2xK8sAvgUtilAutoscalerOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 2500
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 2.0 // Burst to 2x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 8  // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Autoscaler params
		MAX_N_REPLICAS     int     = 4    // At most 2 instances should be needed to handle max load at target utilization
		SCALE_FREQ         int     = 1000 // Make scaling decisions 1/sec
		TARGET_UTIL        float64 = 0.9  // Target utilization of 90%
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE, MAX_N_REPLICAS)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithAvgUtilAutoscaler(asp), opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 5x load burst, with perfect scaling decisions (target utilization
// of 90%) with a short delay, an omniscient qlen-based load balancer, and
// SigmaOS scaling parameters (assuming only cold-starts). 1t = 1ms
func TestBurst5xShortDelayedPerfectScaleOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 750
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 5.0 // Burst to 5x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 8  // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Scaling params
		SCALING_DELAY    uint64  = 100 // "short" autoscaling delay of 100ms
		MAX_N_REPLICAS   int     = 10  // At most 5 instances should be needed to handle max load at target utilization
		TARGET_UTIL      float64 = 0.9 // Target utilization of 90%
		UTIL_WINDOW_SIZE uint64  = 10
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		if time == BURST_START+SCALING_DELAY {
			for i := 0; i < MAX_N_REPLICAS-1; i++ {
				svc.AddInstance()
			}
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 5x load burst, with perfect scaling decisions (target utilization
// of 90%) with a long delay, an omniscient qlen-based load balancer, and
// SigmaOS scaling parameters (assuming only cold-starts). 1t = 1ms
func TestBurst5xLongDelayedPerfectScaleOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 5000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 5.0 // Burst to 5x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 8  // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		// Scaling params
		SCALING_DELAY    uint64  = 1000 // "long" autoscaling delay of 1000ms
		MAX_N_REPLICAS   int     = 10   // At most 5 instances should be needed to handle max load at target utilization
		TARGET_UTIL      float64 = 0.9  // Target utilization of 90%
		UTIL_WINDOW_SIZE uint64  = 10
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric())
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		if time == BURST_START+SCALING_DELAY {
			for i := 0; i < MAX_N_REPLICAS-1; i++ {
				svc.AddInstance()
			}
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 5x load burst, with perfect scaling decisions (target utilization
// of 90%) with a long delay, an omniscient qlen-based load balancer, and
// SigmaOS scaling parameters (assuming only cold-starts). 1t = 1ms
func TestBurst5xLongDelayedMaxQDelayQMGrPerfectScaleOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 2000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 5.0 // Burst to 5x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 8  // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		MAX_Q_DELAY         uint64 = 100 // Max queueing delay incurred by any request before it is retried (possibly at another replica)
		// Scaling params
		SCALING_DELAY    uint64  = 1000 // "long" autoscaling delay of 1000ms
		MAX_N_REPLICAS   int     = 10   // At most 5 instances should be needed to handle max load at target utilization
		TARGET_UTIL      float64 = 0.9  // Target utilization of 90%
		UTIL_WINDOW_SIZE uint64  = 10
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric(), opts.WithMaxQDelayQMgr(MAX_Q_DELAY))
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		if time == BURST_START+SCALING_DELAY {
			for i := 0; i < MAX_N_REPLICAS-1; i++ {
				svc.AddInstance()
			}
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

// Simulate a 5x load burst. Overscale to drain built-up queues after a medium
// delay, an omniscient qlen-based load balancer, and SigmaOS scaling
// parameters (assuming only cold-starts). 1t = 1ms
func TestBurst5xMaxQDelayQMGrOverscaleLongSvcInitOmniscientQLenLBSigmaOSColdStartParams(t *testing.T) {
	const (
		N_TICKS uint64 = 750
		// Clnt params
		CLNT_REQ_MEAN    float64 = 9 // 9 requests per ms
		CLNT_REQ_STD     float64 = 0
		BURST_MULTIPLIER float64 = 5.0 // Burst to 5x the load
		BURST_START      uint64  = 100 // Start burst 100 ticks in
		// App params
		N_SLOTS             int    = 50 // With 9 requests per millisecond, and 5ms to process each request, a server with 50 processing slots should achieve 90% avg utilization.
		P_TIME              uint64 = 5  // Request processing time is 5ms, which is in-line with many hotel RPCs
		INIT_TIME           uint64 = 50 // SigmaOS cold-start time, for just the container, is ~7.5ms. Real time to serve requests would be slighlty longer, due to the need to e.g. establish connections, register in the namespace, etc. This is therefore certainly a lower-bound
		SVC_ID              string = "wfe"
		STATEFUL            bool   = false
		RECORD_STATS_WINDOW int    = 10
		MAX_Q_DELAY         uint64 = 50 // Max queueing delay incurred by any request before it is retried (possibly at another replica)
		// Scaling params
		SCALING_DELAY    uint64  = 50  // "short" autoscaling delay of 50ms
		MAX_N_REPLICAS   int     = 20  // At most 5 instances should be needed to handle max load at target utilization
		TARGET_UTIL      float64 = 0.9 // Target utilization of 90%
		UTIL_WINDOW_SIZE uint64  = 10
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, INIT_TIME, STATEFUL)
	svc := simms.NewMicroservice(&time, p, opts.DefaultMicroserviceOpts, opts.WithOmniscientLB(), opts.WithLoadBalancerQLenMetric(), opts.WithMaxQDelayQMgr(MAX_Q_DELAY))
	app := simms.NewSingleTierApp(svc)
	w := simms.NewWorkload(&time, app, c)
	w.RecordStats(RECORD_STATS_WINDOW)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		if time == BURST_START+SCALING_DELAY {
			for i := 0; i < MAX_N_REPLICAS-1; i++ {
				svc.AddInstance()
			}
		}
		// Run the simulation
		w.Tick()
	}
	stats := w.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Verbose Latency stats over time:\n%v", rstats.VerboseString())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	//	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	//	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "nreqs:%v nreps:%v", svc.GetNReqs(), stats.GetNReps())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}
