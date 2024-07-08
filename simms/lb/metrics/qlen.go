package metrics

import (
	"sigmaos/simms"
)

type QLen struct {
	steeredReqs [][]*simms.Request
	instances   []*simms.MicroserviceInstance
}

func NewQLenMetric(steeredReqs [][]*simms.Request, instances []*simms.MicroserviceInstance) Metric {
	return &QLen{
		steeredReqs: steeredReqs,
		instances:   instances,
	}
}

func (m *QLen) Less(i, j int) bool {
	iQLen := m.instances[i].GetQLen() + len(m.steeredReqs[i])
	jQLen := m.instances[j].GetQLen() + len(m.steeredReqs[j])
	return iQLen < jQLen
}
