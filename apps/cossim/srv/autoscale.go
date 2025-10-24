package cossim

import (
	"fmt"

	"sigmaos/apps/cossim/clnt"
	"sigmaos/autoscale"
	db "sigmaos/debug"
)

type RequestsInFlightMetric struct {
	rif  float64
	cssc *clnt.CosSimShardClnt
}

func NewRequestsInFlightMetric(cssc *clnt.CosSimShardClnt) autoscale.Metric {
	return &RequestsInFlightMetric{
		rif:  0.0,
		cssc: cssc,
	}
}

func (rif *RequestsInFlightMetric) update() {
	metrics, err := rif.cssc.GetAllServerMetrics()
	if err != nil {
		db.DPrintf(db.AUTOSCALER_ERR, "Err GetAllServerMetrics: %v", err)
		return
	}
	if len(metrics) == 0 {
		rif.rif = 0.0
		return
	}
	var total uint64
	for _, m := range metrics {
		total += m.RIF
	}
	rif.rif = float64(total) / float64(len(metrics))
}

func (rif *RequestsInFlightMetric) GetValue() float64 {
	rif.update()
	return rif.rif
}

func (rif *RequestsInFlightMetric) String() string {
	return fmt.Sprintf("&{ rif:%v }", rif.rif)
}
