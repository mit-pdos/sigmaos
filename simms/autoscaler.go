package simms

type NewAutoscalerFn func(*Microservice) Autoscaler

type Autoscaler interface {
	Tick()
	Start()
	Stop()
	NScaleUpEvents() int
	NScaleDownEvents() int
}
