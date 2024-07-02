package simms

type Autoscaler interface {
	Tick()
	Start()
	Stop()
	NScaleUpEvents() int
	NScaleDownEvents() int
}

// Autoscaler which tries to maintain selected utilization level
//
// TODO: make AIMD autoscaler
type AvgUtilAutoscaler struct {
	t              *uint64
	svc            *Microservice
	scaleFrequency int
	run            bool
	nScaleUp       int
	nScaleDown     int
}

// Target resource utilization, as a percentage [0.0, 100.0]
func NewAvgUtilAutoscaler(t *uint64, freq int, targetUtil float64, svc *Microservice) *AvgUtilAutoscaler {
	return &AvgUtilAutoscaler{
		t:              t,
		svc:            svc,
		scaleFrequency: freq,
		run:            false,
		nScaleUp:       0,
		nScaleDown:     0,
	}
}

func (ua *AvgUtilAutoscaler) Tick() {
	// If autoscaler is not running, bail out
	if !ua.run {
		return
	}
	// Only scale every scaleFrequency ticks
	if *ua.t%uint64(ua.scaleFrequency) != 0 {
		return
	}
	istats := ua.svc.GetInstanceStats()
	_ = istats
}

func (ua *AvgUtilAutoscaler) Start() {
	ua.run = true
}

func (ua *AvgUtilAutoscaler) Stop() {
	ua.run = false
}
