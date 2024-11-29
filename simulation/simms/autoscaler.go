package simms

type NewAutoscalerFn func(*uint64, *Microservice) Autoscaler

type Autoscaler interface {
	Tick()
	Start()
	Stop()
	NScaleUpEvents() int
	NScaleDownEvents() int
}
