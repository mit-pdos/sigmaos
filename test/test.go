package test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
)

type Tstate struct {
	sync.Mutex
	wg sync.WaitGroup
	T  *testing.T
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

func (ts *Tstate) addNamedReplica(i int) {
	defer ts.wg.Done()
	r := kernel.MakeSystemNamed("test", "..", i)
	ts.Lock()
	defer ts.Unlock()
	ts.replicas = append(ts.replicas, r)
}

func (ts *Tstate) startReplicas() {
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		// Needs to happen in a separate thread because MakeSystemNamed will block until the replicas are able to process requests.
		go ts.addNamedReplica(i + 1)
	}
}

func MakeTstatePath(t *testing.T, named, path string) *Tstate {
	if named == "" && path == np.NAMED {
		return MakeTstate(t)
	} else {
		var ts *Tstate
		if named == "" {
			ts = MakeTstateAll(t)
		} else {
			ts = MakeTstateClnt(t, named)
		}
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts
	}
}

func MakeTstateClnt(t *testing.T, named string) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.System = kernel.MakeSystemClnt("test", named)
	return ts
}

func MakeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.wg.Add(len(fslib.Named()))
	// Needs to happen in a separate thread because MakeSystem will block until enough replicas have started (if named is replicated).
	go func() {
		defer ts.wg.Done()
		ts.System = kernel.MakeSystemNamed("test", "..", 0)
	}()
	ts.startReplicas()
	ts.wg.Wait()
	return ts
}

func MakeTstateAll(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.wg.Add(len(fslib.Named()))
	// Needs to happen in a separate thread because MakeSystem will block until enough replicas have started (if named is replicated).
	go func() {
		defer ts.wg.Done()
		ts.System = kernel.MakeSystemAll("test", "..", 0)
	}()
	ts.startReplicas()
	ts.wg.Wait()
	return ts
}

func MakeTstateAllBin(t *testing.T, bin string) *Tstate {
	ts := &Tstate{}
	ts.T = t
	ts.wg.Add(len(fslib.Named()))
	// Needs to happen in a separate thread because MakeSystem will block until enough replicas have started (if named is replicated).
	go func() {
		defer ts.wg.Done()
		ts.System = kernel.MakeSystemAll("test", bin, 0)
	}()
	ts.startReplicas()
	ts.wg.Wait()
	return ts
}

const (
	MBYTE = 1 << 20
)

func Mbyte(sz np.Tlength) float64 {
	return float64(sz) / float64(MBYTE)
}

func Tput(sz np.Tlength, ms int64) float64 {
	s := float64(ms) / 1000
	return Mbyte(sz) / s
}
