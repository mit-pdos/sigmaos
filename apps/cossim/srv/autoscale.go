package cossim

import (
	"fmt"

	"sigmaos/autoscale"
)

type RequestsInFlightMetric struct {
	rif float64
}

func NewRequestsInFlightMetric() autoscale.Metric {
	return &RequestsInFlightMetric{
		rif: 0.0,
	}
}

func (rif *RequestsInFlightMetric) update() {
	// TODO
}

func (rif *RequestsInFlightMetric) GetValue() float64 {
	rif.update()
	return rif.rif
}

func (rif *RequestsInFlightMetric) String() string {
	return fmt.Sprintf("&{ rif:%v }", rif.rif)
}
