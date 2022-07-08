package test

import (
	"flag"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
)

var version string

// Read & set the proc version.
func init() {
	flag.StringVar(&version, "version", "none", "version")
}

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
	r := kernel.MakeSystemNamed("test", np.TEST_RID, i, np.MkInterval(0, np.Toffset(linuxsched.NCores)))
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
	setVersion()
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
	setVersion()
	ts := &Tstate{}
	ts.T = t
	ts.System = kernel.MakeSystem("test", np.TEST_RID, []string{named}, np.MkInterval(0, np.Toffset(linuxsched.NCores)))
	return ts
}

func MakeTstate(t *testing.T) *Tstate {
	setVersion()
	ts := &Tstate{}
	ts.T = t
	ts.makeSystem(kernel.MakeSystemNamed)
	return ts
}

func MakeTstateAll(t *testing.T) *Tstate {
	setVersion()
	ts := &Tstate{}
	ts.T = t
	ts.makeSystem(kernel.MakeSystemAll)
	return ts
}

func (ts *Tstate) makeSystem(mkSys func(string, string, int, *np.Tinterval) *kernel.System) {
	ts.wg.Add(len(fslib.Named()))
	// Needs to happen in a separate thread because MakeSystem will block until enough replicas have started (if named is replicated).
	go func() {
		defer ts.wg.Done()
		ts.System = mkSys("test", np.TEST_RID, 0, np.MkInterval(0, np.Toffset(linuxsched.NCores)))
	}()
	ts.startReplicas()
	ts.wg.Wait()
}

func setVersion() {
	if version == "" || version == "none" || !flag.Parsed() {
		db.DFatalf("Version not set in test")
	}
	proc.Version = version
}

const (
	MBYTE = 1 << 20
	BUFSZ = 1 << 16
)

func Mbyte(sz np.Tlength) float64 {
	return float64(sz) / float64(MBYTE)
}

func Tput(sz np.Tlength, ms int64) string {
	s := float64(ms) / 1000
	return fmt.Sprintf("%.2fMB/s", Mbyte(sz)/s)
}
