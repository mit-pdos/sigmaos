package procdep

import (
	"fmt"

	"ulambda/proc"
)

type ProcDep struct {
	Started      bool
	Dependencies *Deps // ProcDeps which this procDep depends on
	Dependants   *Deps // ProcDeps which depend on this procDep
	*proc.Proc
}

type Deps struct {
	StartDep map[string]bool
	ExitDep  map[string]bool
}

func MakeDeps(start, end map[string]bool) *Deps {
	return &Deps{start, end}
}

func MakeProcDep() *ProcDep {
	t := &ProcDep{}
	t.Dependencies = MakeDeps(map[string]bool{}, map[string]bool{})
	t.Dependants = MakeDeps(map[string]bool{}, map[string]bool{})
	t.Proc = &proc.Proc{}
	return t
}

func (p *ProcDep) GetProc() *proc.Proc {
	return p.Proc
}

func (p *ProcDep) String() string {
	return fmt.Sprintf("&{ proc:%v started:%v, dependencies:%v, dependants:%v }", p.Proc, p.Started, p.Dependencies, p.Dependants)
}

func (d *Deps) String() string {
	return fmt.Sprintf("&{ start:%v exit:%v }", d.StartDep, d.ExitDep)
}
