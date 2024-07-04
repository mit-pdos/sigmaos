package autoscaler

import (
	db "sigmaos/debug"
	"sigmaos/simms"
)

func GetNewAvgUtilAutoscalerFn(asp *AvgUtilAutoscalerParams) simms.NewAutoscalerFn {
	return func(t *uint64, svc *simms.Microservice) simms.Autoscaler {
		return NewAvgUtilAutoscaler(t, asp, svc)
	}
}

type AvgUtilAutoscalerParams struct {
	ScaleFreq      int
	TargetUtil     float64
	UtilWindowSize uint64
}

func NewAvgUtilAutoscalerParams(scaleFreq int, targetUtil float64, utilWindowSize uint64) *AvgUtilAutoscalerParams {
	return &AvgUtilAutoscalerParams{
		ScaleFreq:      scaleFreq,
		TargetUtil:     targetUtil,
		UtilWindowSize: utilWindowSize,
	}
}

// Autoscaler which tries to maintain selected average utilization level
type AvgUtilAutoscaler struct {
	t          *uint64
	svc        *simms.Microservice
	p          *AvgUtilAutoscalerParams
	ctx        *Ctx
	run        bool
	nScaleUp   int
	nScaleDown int
}

// Target resource utilization, as a percentage [0.0, 100.0]
func NewAvgUtilAutoscaler(t *uint64, asp *AvgUtilAutoscalerParams, svc *simms.Microservice) *AvgUtilAutoscaler {
	return &AvgUtilAutoscaler{
		t:          t,
		svc:        svc,
		p:          asp,
		ctx:        NewCtx(t, svc.GetID()),
		run:        false,
		nScaleUp:   0,
		nScaleDown: 0,
	}
}

func (ua *AvgUtilAutoscaler) Tick() {
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
	db.DPrintf(db.SIM_AUTOSCALE, "%v Run AvgUtilAutoscaler", ua.ctx)
	istats := ua.svc.GetInstanceStats()
	d, n := ua.getScalingDecision(istats)
	db.DPrintf(db.SIM_AUTOSCALE, "%v AvgUtilAutoscaler scaling decision (%v, %v)", ua.ctx, d, n)
	switch d {
	case SCALE_UP:
		for i := 0; i < n; i++ {
			ua.svc.AddReplica()
		}
		ua.nScaleUp++
	case SCALE_DOWN:
		for i := 0; i < n; i++ {
			ua.svc.RemoveReplica()
		}
		ua.nScaleDown++
	default:
	}
}

func (ua *AvgUtilAutoscaler) getScalingDecision(istats []*simms.ServiceInstanceStats) (scalingDecision, int) {
	readyIStats := getReadyInstanceStats(*ua.t, istats)
	currentNReplicas := len(readyIStats)
	currentUtil := avgUtil(ua.ctx, *ua.t, ua.p.UtilWindowSize, readyIStats)
	desiredNReplicas := k8sCalcDesiredNReplicas(ua.ctx, currentNReplicas, currentUtil, ua.p.TargetUtil, DEFAULT_TOLERANCE)
	db.DPrintf(db.SIM_AUTOSCALE, "%v AvgUtilAutoscaler currentUtil:%v targetUtil:%v, currentNReplicas:%v desiredNReplicas:%v", ua.ctx, currentUtil, ua.p.TargetUtil, currentNReplicas, desiredNReplicas)
	if desiredNReplicas > currentNReplicas {
		return SCALE_UP, desiredNReplicas - currentNReplicas
	}
	if desiredNReplicas < currentNReplicas {
		return SCALE_DOWN, currentNReplicas - desiredNReplicas
	}
	return SCALE_NONE, 0
}

func (ua *AvgUtilAutoscaler) Start() {
	ua.run = true
	db.DPrintf(db.SIM_AUTOSCALE, "%v Start AvgUtilAutoscaler", ua.ctx)
}

func (ua *AvgUtilAutoscaler) Stop() {
	ua.run = false
	db.DPrintf(db.SIM_AUTOSCALE, "%v Stop AvgUtilAutoscaler", ua.ctx)
}

func (ua *AvgUtilAutoscaler) NScaleUpEvents() int {
	return ua.nScaleUp
}

func (ua *AvgUtilAutoscaler) NScaleDownEvents() int {
	return ua.nScaleDown
}
