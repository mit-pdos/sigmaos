package procd

import (
	db "sigmaos/debug"
	"sigmaos/mem"
	"sigmaos/proc"
)

func (pd *Procd) hasEnoughMemL(p *proc.Proc) bool {
	return pd.memAvail >= p.GetMem()
}

func (pd *Procd) allocMemL(p *proc.Proc) {
	pd.memAvail -= p.GetMem()
	pd.sanityCheckMemL()
}

func (pd *Procd) freeMem(p *proc.Proc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.memAvail += p.GetMem()
	pd.sanityCheckMemL()
}

func (pd *Procd) sanityCheckMemL() {
	if pd.memAvail < 0 || pd.memAvail > mem.GetTotalMem() {
		db.DFatalf("Memory sanity check failed %v", pd.memAvail)
	}
}
