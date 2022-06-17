package machine

import (
	"fmt"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
)

// XXX eventually, once our allocation strategy gets more complex, we'll
// probably want to use ulambda/intervals here.
type MachineConfig struct {
	MaxCores  uint
	FreeCores *np.Tinterval
}

func MakeMachineConfig(Ncores uint) *MachineConfig {
	cfg := &MachineConfig{}
	cfg.MaxCores = Ncores
	cfg.FreeCores = np.MkInterval(0, np.Toffset(Ncores+1))
	return cfg
}

// Allocate a core interval and return it. Currently, for simplicity, this
// assumes that cores are allocated and freed in contiguous segments.
func (cfg *MachineConfig) AllocCores(n proc.Tcore) *np.Tinterval {
	if cfg.FreeCores.Size() < np.Tsize(n) {
		db.DFatalf("Tried to alloc more cores (%v) than are available: %v", n, cfg.FreeCores)
	}
	oldStart := cfg.FreeCores.Start
	newStart := cfg.FreeCores.Start + np.Toffset(n)
	cfg.FreeCores.Start = newStart
	return np.MkInterval(oldStart, newStart+1)
}

// Free a core interval. Currently, for simplicity, this assumes that cores are
// allocated and freed in contiguous segments.
func (cfg *MachineConfig) FreeCoreIV(iv *np.Tinterval) {
	// Make sure the intervals don't overlap
	if (iv.End < cfg.FreeCores.End && iv.End > cfg.FreeCores.Start) || (iv.Start < cfg.FreeCores.End && iv.Start >= cfg.FreeCores.Start) {
		db.DFatalf("Double free, iv %v overlaps with FreeCores %v", iv, cfg.FreeCores)
	}
	if iv.Start < cfg.FreeCores.Start {
		cfg.FreeCores.Start = iv.Start
	}
	if iv.End > cfg.FreeCores.End {
		cfg.FreeCores.End = iv.End
	}
	// Make sure we didn't free too many cores.
	if uint(cfg.FreeCores.Size()) > cfg.MaxCores {
		db.DFatalf("Freed too many cores: have more FreeCores %v than MaxCores %v", cfg.FreeCores, cfg.MaxCores)
	}
}

func (cfg *MachineConfig) String() string {
	return fmt.Sprintf("&{ MaxCores:%v FreeCores:%v }", cfg.MaxCores, cfg.FreeCores)
}
