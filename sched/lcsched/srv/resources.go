package srv

import (
	db "sigmaos/debug"
	"sigmaos/proc"
)

type Resources struct {
	maxmcpu proc.Tmcpu
	maxmem  proc.Tmem
	mcpu    proc.Tmcpu
	mem     proc.Tmem
}

func newResources(mcpuInt uint32, memInt uint32) *Resources {
	return &Resources{
		maxmcpu: proc.Tmcpu(mcpuInt),
		maxmem:  proc.Tmem(memInt),
		mcpu:    proc.Tmcpu(mcpuInt),
		mem:     proc.Tmem(memInt),
	}
}

// Caller holds lock
func (r *Resources) alloc(p *proc.Proc) {
	defer r.sanityCheck()

	r.mcpu -= p.GetMcpu()
	r.mem -= p.GetMem()
}

// Caller holds lock
func (r *Resources) free(p *proc.Proc) {
	defer r.sanityCheck()

	r.mcpu += p.GetMcpu()
	r.mem += p.GetMem()
}

func (r *Resources) sanityCheck() {
	if r.mcpu < 0 || r.mem < 0 {
		db.DFatalf("Invalid mcpu (%v) or mem (%v): too little", r.mcpu, r.mem)
	}
	if r.mcpu > r.maxmcpu || r.mem > r.maxmem {
		db.DFatalf("Invalid mcpu (%v) or mem (%v): too much", r.mcpu, r.mem)
	}
}
