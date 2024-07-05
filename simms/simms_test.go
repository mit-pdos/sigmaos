package simms_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/simms"
	"sigmaos/simms/autoscaler"
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
	svc := simms.NewServiceInstance(&time, p, 0)
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
	svc := simms.NewServiceInstance(&time, p, 0)
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
	svc := simms.NewServiceInstance(&time, p, 0)
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
	svc := simms.NewServiceInstance(&time, p, 0)
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
	svc := simms.NewServiceInstance(&time, p, 0)
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
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewNoOpAutoscalerFn())
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
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewNoOpAutoscalerFn())
	app := simms.NewSingleTierApp(svc)
	dc := simms.NewWorkload(&time, app, c)
	for ; time < N_TICKS; time++ {
		// Run the simulation
		dc.Tick()
	}
	stats := dc.GetStats()
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
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewNoOpAutoscalerFn())
	app := simms.NewSingleTierApp(svc)
	dc := simms.NewWorkload(&time, app, c)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
		}
		if time == BURST_END {
			c.EndBurst()
		}
		// Run the simulation
		dc.Tick()
	}
	stats := dc.GetStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestWorkloadClntBurstAddReplica(t *testing.T) {
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
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewNoOpAutoscalerFn())
	app := simms.NewSingleTierApp(svc)
	dc := simms.NewWorkload(&time, app, c)
	for ; time < N_TICKS; time++ {
		if time == BURST_START {
			c.StartBurst(BURST_MULTIPLIER)
			svc.AddReplica()
		}
		if time == BURST_END {
			c.EndBurst()
		}
		// Run the simulation
		dc.Tick()
	}
	stats := dc.GetStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	assert.Equal(t, float64(1.0), stats.AvgLatency())
	db.DPrintf(db.SIM_TEST, "Sim test done")
}

func TestWorkloadClntBurstRemoveReplica(t *testing.T) {
	const (
		N_TICKS uint64 = 1000
		// Clnt params
		CLNT_REQ_MEAN    float64 = 1
		CLNT_REQ_STD     float64 = 0
		BURST_START      uint64  = 0
		BURST_END        uint64  = 1000
		BURST_MULTIPLIER float64 = 2.0
		// App params
		N_SLOTS        int    = 1
		P_TIME         uint64 = 1
		SVC_ID         string = "wfe"
		STATEFUL       bool   = false
		SIZE_UP_TIME   uint64 = 0
		SIZE_DOWN_TIME uint64 = 500
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewNoOpAutoscalerFn())
	app := simms.NewSingleTierApp(svc)
	dc := simms.NewWorkload(&time, app, c)
	dc.RecordStats(10)
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
			svc.AddReplica()
		}
		if time == SIZE_DOWN_TIME {
			svc.RemoveReplica()
		}
		// Run the simulation
		dc.Tick()
	}
	stats := dc.GetStats()
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
		N_SLOTS        int    = 1
		P_TIME         uint64 = 1
		SVC_ID         string = "wfe"
		STATEFUL       bool   = false
		SIZE_UP_TIME   uint64 = 0
		SIZE_DOWN_TIME uint64 = 500
		// Autoscaler params
		SCALE_FREQ         int     = 10
		TARGET_UTIL        float64 = 0.5
		UTIL_WINDOW_SIZE   uint64  = 10
		AUTOSCALER_LEAD_IN uint64  = 100 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE)
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewAvgUtilAutoscalerFn(asp))
	app := simms.NewSingleTierApp(svc)
	dc := simms.NewWorkload(&time, app, c)
	dc.RecordStats(10)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		dc.Tick()
	}
	stats := dc.GetStats()
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
		N_SLOTS        int    = 10
		P_TIME         uint64 = 2
		SVC_ID         string = "wfe"
		STATEFUL       bool   = false
		SIZE_UP_TIME   uint64 = 0
		SIZE_DOWN_TIME uint64 = 500
		// Autoscaler params
		SCALE_FREQ         int     = 1
		TARGET_UTIL        float64 = 0.9
		UTIL_WINDOW_SIZE   uint64  = 1
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE)
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewAvgUtilAutoscalerFn(asp))
	app := simms.NewSingleTierApp(svc)
	dc := simms.NewWorkload(&time, app, c)
	dc.RecordStats(10)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		dc.Tick()
	}
	stats := dc.GetStats()
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
		N_SLOTS        int    = 10
		P_TIME         uint64 = 2
		SVC_ID         string = "wfe"
		STATEFUL       bool   = false
		SIZE_UP_TIME   uint64 = 0
		SIZE_DOWN_TIME uint64 = 500
		// Autoscaler params
		SCALE_FREQ         int     = 1
		TARGET_UTIL        float64 = 0.5
		UTIL_WINDOW_SIZE   uint64  = 1
		AUTOSCALER_LEAD_IN uint64  = 10 // Number of ticks to wait before starting the autoscaler
	)
	db.DPrintf(db.SIM_TEST, "Sim test start")
	var time uint64 = 0
	c := simms.NewClients(CLNT_REQ_MEAN, CLNT_REQ_STD)
	p := simms.NewMicroserviceParams(SVC_ID, N_SLOTS, P_TIME, 0, STATEFUL)
	asp := autoscaler.NewAvgUtilAutoscalerParams(SCALE_FREQ, TARGET_UTIL, UTIL_WINDOW_SIZE)
	svc := simms.NewMicroservice(&time, p, autoscaler.GetNewAvgUtilAutoscalerFn(asp))
	app := simms.NewSingleTierApp(svc)
	dc := simms.NewWorkload(&time, app, c)
	dc.RecordStats(10)
	for ; time < N_TICKS; time++ {
		if time == AUTOSCALER_LEAD_IN {
			svc.GetAutoscaler().Start()
		}
		// Run the simulation
		dc.Tick()
	}
	stats := dc.GetStats()
	rstats := stats.GetRecordedStats()
	db.DPrintf(db.SIM_TEST, "Avg latency: %v", stats.AvgLatency())
	db.DPrintf(db.SIM_RAW_LAT, "Raw latency: %v", stats.GetLatencies())
	db.DPrintf(db.SIM_LAT_STATS, "Latency stats over time: %v", rstats)
	assert.Equal(t, 6, svc.GetAutoscaler().NScaleUpEvents(), "Scaled up wrong number of times")
	assert.Equal(t, 2, svc.GetAutoscaler().NScaleDownEvents(), "Scaled down wrong number of times")
	db.DPrintf(db.SIM_TEST, "Sim test done")
}
