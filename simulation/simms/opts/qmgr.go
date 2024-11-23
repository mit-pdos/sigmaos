package opts

import (
	"sigmaos/simms"
	"sigmaos/simms/qmgr"
)

type withBasicQMgr struct{}

func (withBasicQMgr) Apply(opts *simms.MicroserviceOpts) {
	opts.NewQMgr = qmgr.NewBasicQMgr
}

func WithBasicQMgr() simms.MicroserviceOpt {
	return &withBasicQMgr{}
}

type withMaxQDelayQMgr struct {
	maxDelay uint64
}

func (o withMaxQDelayQMgr) Apply(opts *simms.MicroserviceOpts) {
	opts.NewQMgr = func(t *uint64, ms *simms.Microservice) simms.QMgr {
		return qmgr.NewMaxQDelayQMgr(t, o.maxDelay, ms)
	}
}

func WithMaxQDelayQMgr(maxDelay uint64) simms.MicroserviceOpt {
	return &withMaxQDelayQMgr{
		maxDelay: maxDelay,
	}
}
