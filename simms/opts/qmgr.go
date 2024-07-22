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
	clnts    *simms.Clients
	maxDelay uint64
}

func (o withMaxQDelayQMgr) Apply(opts *simms.MicroserviceOpts) {
	opts.NewQMgr = func(t *uint64) simms.QMgr {
		return qmgr.NewMaxQDelayQMgr(t, o.maxDelay, o.clnts)
	}
}

func WithMaxQDelayQMgr(maxDelay uint64, clnts *simms.Clients) simms.MicroserviceOpt {
	return &withMaxQDelayQMgr{
		clnts:    clnts,
		maxDelay: maxDelay,
	}
}
