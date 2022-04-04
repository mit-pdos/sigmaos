package test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
)

type Tstate struct {
	T *testing.T
	*kernel.System
	replicas []*kernel.System
}

func (ts *Tstate) Shutdown() {
	db.DPrintf("TEST", "Shutting down")
	ts.System.Shutdown()
	for _, r := range ts.replicas {
		r.Shutdown()
	}
	N := 30 // Crashing procds in mr test leave several fids open; maybe too many?
	assert.True(ts.T, ts.PathClnt.FidClnt.Len() < N, ts.PathClnt.FidClnt)
	db.DPrintf("TEST", "Done shutting down")
}

func (ts *Tstate) startReplicas() {
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		ts.replicas = append(ts.replicas, kernel.MakeSystemNamed("test", "..", i+1))
	}
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	if path == np.NAMED {
		return MakeTstate(t)
	} else {
		ts := MakeTstateAll(t)
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts
	}
}

func MakeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.System = kernel.MakeSystemNamed("test", "..", 0)
	ts.startReplicas()
	return ts
}

func MakeTstateAll(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.System = kernel.MakeSystemAll("test", "..", 0)
	ts.startReplicas()
	return ts
}

func MakeTstateAllBin(t *testing.T, bin string) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.System = kernel.MakeSystemAll("test", bin, 0)
	ts.startReplicas()
	return ts
}
