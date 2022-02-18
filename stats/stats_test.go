package stats_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/stats"
)

type Tstate struct {
	t *testing.T
	*kernel.System
	replicas []*kernel.System
}

func (ts *Tstate) Shutdown() {
	ts.System.Shutdown()
	for _, r := range ts.replicas {
		r.Shutdown()
	}
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemNamed("statstest", "..", 0)
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		ts.replicas = append(ts.replicas, kernel.MakeSystemNamed("fslibtest", "..", i+1))
	}
	return ts
}

func TestStatsd(t *testing.T) {
	ts := makeTstate(t)

	st := stats.StatInfo{}
	err := ts.ReadFileJson("name/statsd", &st)
	assert.Nil(t, err, "statsd")
	assert.Equal(t, stats.Tcounter(0), st.Nread, "Nread")
	for i := 0; i < 1000; i++ {
		_, err := ts.ReadFile("name/statsd")
		assert.Nil(t, err, "statsd")
	}
	err = ts.ReadFileJson("name/statsd", &st)
	assert.Nil(t, err, "statsd")
	assert.Equal(t, st.Nopen, stats.Tcounter(1000), "statsd")

	last := float64(0.0)
	for i := 0; i < 5; i++ {
		err = ts.ReadFileJson("name/statsd", &st)
		assert.Nil(t, err, "statsd")
		assert.NotEqual(t, last, st.Util, "util")
		last = st.Util
		time.Sleep(100 * time.Millisecond)
	}

	ts.Shutdown()
}
