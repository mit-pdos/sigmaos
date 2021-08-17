package depproc

import (
	"fmt"

	"ulambda/proc"
)

type DepProc struct {
	Started      bool
	Dependencies *Deps // DepProcs which this depProc depends on
	Dependants   *Deps // DepProcs which depend on this depProc
	*proc.Proc
}

type Deps struct {
	StartDep map[string]bool
	ExitDep  map[string]bool
}

func MakeDeps(start, end map[string]bool) *Deps {
	return &Deps{start, end}
}

func MakeDepProc() *DepProc {
	t := &DepProc{}
	t.Dependencies = MakeDeps(map[string]bool{}, map[string]bool{})
	t.Dependants = MakeDeps(map[string]bool{}, map[string]bool{})
	t.Proc = &proc.Proc{}
	return t
}

func (t *DepProc) String() string {
	return fmt.Sprintf("&{ proc:%v started:%v, dependencies:%v, dependants:%v }", t.Proc, t.Started, t.Dependencies, t.Dependants)
}

func (d *Deps) String() string {
	return fmt.Sprintf("&{ start:%v exit:%v }", d.StartDep, d.ExitDep)
}
