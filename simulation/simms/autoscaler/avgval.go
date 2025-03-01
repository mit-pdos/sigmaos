package autoscaler

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

const (
	UNLIMITED_REPLICAS int = 0
)

type avgValFn func(ctx *Ctx, currentT uint64, windowSize uint64, istats []*simms.ServiceInstanceStats) float64

type AvgValAutoscalerParams struct {
	avgVal        avgValFn
	ScaleFreq     int
	TargetVal     float64
	ValWindowSize uint64
	maxNReplicas  int
}

func NewAvgValAutoscalerParams(scaleFreq int, targetVal float64, valWindowSize uint64, maxNReplicas int, avgVal avgValFn) *AvgValAutoscalerParams {
	return &AvgValAutoscalerParams{
		avgVal:        avgVal,
		ScaleFreq:     scaleFreq,
		TargetVal:     targetVal,
		ValWindowSize: valWindowSize,
		maxNReplicas:  maxNReplicas,
	}
}

// Autoscaler which tries to maintain selected average valization level
type AvgValAutoscaler struct {
	t          *uint64
	svc        *simms.Microservice
	p          *AvgValAutoscalerParams
	ctx        *Ctx
	run        bool
	nScaleUp   int
	nScaleDown int
}

// Target resource valization, as a percentage [0.0, 100.0]
func NewAvgValAutoscaler(t *uint64, asp *AvgValAutoscalerParams, svc *simms.Microservice) *AvgValAutoscaler {
	return &AvgValAutoscaler{
		t:          t,
		svc:        svc,
		p:          asp,
		ctx:        NewCtx(t, svc.GetID()),
		run:        false,
		nScaleUp:   0,
		nScaleDown: 0,
	}
}

func (ua *AvgValAutoscaler) Tick() {
	// If autoscaler is not running, bail out
	if !ua.run {
		return
	}
	// Don't try to make a scaling decision as soon as the simulation starts
	if *ua.t == 0 {
		return
	}
	// Only scale every scaleFrequency ticks
	if *ua.t%uint64(ua.p.ScaleFreq) != 0 {
		return
	}
	db.DPrintf(db.SIM_AUTOSCALE, "%v Run AvgValAutoscaler", ua.ctx)
	istats := ua.svc.GetInstanceStats()
	d, n := ua.getScalingDecision(ua.svc.NInstances(), istats)
	db.DPrintf(db.SIM_AUTOSCALE, "%v AvgValAutoscaler scaling decision (%v, %v)", ua.ctx, d, n)
	switch d {
	case SCALE_UP:
		for i := 0; i < n; i++ {
			ua.svc.AddInstance()
		}
		ua.nScaleUp++
	case SCALE_DOWN:
		for i := 0; i < n; i++ {
			ua.svc.RemoveInstance()
		}
		ua.nScaleDown++
	default:
	}
}

func (ua *AvgValAutoscaler) getScalingDecision(currentNInstances int, istats []*simms.ServiceInstanceStats) (scalingDecision, int) {
	readyIStats := getReadyInstanceStats(*ua.t, istats)
	currentVal := ua.p.avgVal(ua.ctx, *ua.t, ua.p.ValWindowSize, readyIStats)
	desiredNInstances := k8sCalcDesiredNInstances(ua.ctx, currentNInstances, currentVal, ua.p.TargetVal, DEFAULT_TOLERANCE, ua.p.maxNReplicas)
	db.DPrintf(db.SIM_AUTOSCALE, "%v AvgValAutoscaler currentVal:%v targetVal:%v, currentNInstances:%v desiredNInstances:%v", ua.ctx, currentVal, ua.p.TargetVal, currentNInstances, desiredNInstances)
	if desiredNInstances > currentNInstances {
		return SCALE_UP, desiredNInstances - currentNInstances
	}
	if desiredNInstances < currentNInstances {
		return SCALE_DOWN, currentNInstances - desiredNInstances
	}
	return SCALE_NONE, 0
}

func (ua *AvgValAutoscaler) Start() {
	ua.run = true
	db.DPrintf(db.SIM_AUTOSCALE, "%v Start AvgValAutoscaler", ua.ctx)
}

func (ua *AvgValAutoscaler) Stop() {
	ua.run = false
	db.DPrintf(db.SIM_AUTOSCALE, "%v Stop AvgValAutoscaler", ua.ctx)
}

func (ua *AvgValAutoscaler) NScaleUpEvents() int {
	return ua.nScaleUp
}

func (ua *AvgValAutoscaler) NScaleDownEvents() int {
	return ua.nScaleDown
}
