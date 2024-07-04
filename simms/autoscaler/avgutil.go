package autoscaler

import (
	"sigmaos/simms"
)

func GetNewAvgUtilAutoscalerFn(t *uint64, asp *AvgUtilAutoscalerParams) simms.NewAutoscalerFn {
	return func(svc *simms.Microservice) simms.Autoscaler {
		return NewAvgUtilAutoscaler(t, asp, svc)
	}
}

type AvgUtilAutoscalerParams struct {
	ScaleFreq      int
	TargetUtil     float64
	UtilWindowSize uint64
}

// Autoscaler which tries to maintain selected average utilization level
type AvgUtilAutoscaler struct {
	t          *uint64
	svc        *simms.Microservice
	p          *AvgUtilAutoscalerParams
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
	istats := ua.svc.GetInstanceStats()
	d, n := ua.getScalingDecision(istats)
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
	currentUtil := avgUtil(*ua.t, ua.p.UtilWindowSize, readyIStats)
	desiredNReplicas := k8sCalcDesiredNReplicas(currentNReplicas, currentUtil, ua.p.TargetUtil, DEFAULT_TOLERANCE)
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
}

func (ua *AvgUtilAutoscaler) Stop() {
	ua.run = false
}

func (ua *AvgUtilAutoscaler) NScaleUpEvents() int {
	return ua.nScaleUp
}

func (ua *AvgUtilAutoscaler) NScaleDownEvents() int {
	return ua.nScaleDown
}
