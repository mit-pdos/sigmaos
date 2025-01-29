package opts

import (
	"sigmaos/simulation/simms"
	"sigmaos/simulation/simms/qmgr"
)

type withBasicQMgr struct {
	maxQLen int
}

func WithBasicQMgr() simms.MicroserviceOpt {
	return &withBasicQMgr{
		maxQLen: 0,
	}
}

func NewBasicQMgr(t *uint64, ms *simms.Microservice) simms.QMgr {
	return qmgr.NewBasicQMgr(t, ms, 0)
}

func (o withBasicQMgr) Apply(opts *simms.MicroserviceOpts) {
	opts.NewQMgr = func(t *uint64, ms *simms.Microservice) simms.QMgr {
		return qmgr.NewBasicQMgr(t, ms, o.maxQLen)
	}
}

func WithMaxQLenQMgr(maxQLen int) simms.MicroserviceOpt {
	return &withBasicQMgr{
		maxQLen: maxQLen,
	}
}

type withMaxQDelayQMgr struct {
	maxDelay uint64
	sorted   bool
}

func (o withMaxQDelayQMgr) Apply(opts *simms.MicroserviceOpts) {
	opts.NewQMgr = func(t *uint64, ms *simms.Microservice) simms.QMgr {
		return qmgr.NewMaxQDelayQMgr(t, o.maxDelay, o.sorted, ms)
	}
}

func WithMaxQDelayQMgr(maxDelay uint64, sorted bool) simms.MicroserviceOpt {
	return &withMaxQDelayQMgr{
		maxDelay: maxDelay,
		sorted:   sorted,
	}
}
