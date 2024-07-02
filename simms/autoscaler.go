package simms

type Autoscaler interface {
	Tick()
	Start()
	Stop()
}

// Autoscaler which tries to maintain selected utilization level
//
// TODO: make AIMD autoscaler
type UtilAutoscaler struct {
	t              *uint64
	svc            *Microservice
	scaleFrequency int
	run            bool
}

// Target resource utilization, as a percentage [0.0, 100.0]
func NewUtilAutoscaler(t *uint64, freq int, targetUtil float64, svc *Microservice) *UtilAutoscaler {
	return &UtilAutoscaler{
		t:              t,
		svc:            svc,
		scaleFrequency: freq,
		run:            false,
	}
}

func (ua *UtilAutoscaler) Tick() {
	// If autoscaler is not running, bail out
	if !ua.run {
		return
	}
	// Only scale every scaleFrequency ticks
	if *ua.t%uint64(ua.scaleFrequency) == 0 {
	}
}

func (ua *UtilAutoscaler) Start() {
	ua.run = true
}

func (ua *UtilAutoscaler) Stop() {
	ua.run = false
}
