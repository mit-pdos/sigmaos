package opts

import (
	"sigmaos/simulation/simms"
)

func WithKillRemovedInstances() simms.MicroserviceOpt {
	return &withKillRemovedInstances{}
}

type withKillRemovedInstances struct{}

func (withKillRemovedInstances) Apply(opts *simms.MicroserviceOpts) {
	opts.KillRemovedInstances = true
}
