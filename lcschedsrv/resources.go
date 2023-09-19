package lcschedsrv

import (
	"sigmaos/proc"
)

type Resources struct {
	mcpu proc.Tmcpu
	mem  proc.Tmem
}

func newResources(mcpuInt uint32, memInt uint32) *Resources {
	return &Resources{
		mcpu: proc.Tmcpu(mcpuInt),
		mem:  proc.Tmem(memInt),
	}
}
