package test

import (
	"testing"

	"ulambda/fslib"
	"ulambda/kernel"
)

type Tstate struct {
	T *testing.T
	*kernel.System
	replicas []*kernel.System
}

func (ts *Tstate) Shutdown() {
	ts.System.Shutdown()
	for _, r := range ts.replicas {
		r.Shutdown()
	}
}

func MakeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.System = kernel.MakeSystemNamed("fslibtest", "..", 0)
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		ts.replicas = append(ts.replicas, kernel.MakeSystemNamed("fslibtest", "..", i+1))
	}
	return ts
}
