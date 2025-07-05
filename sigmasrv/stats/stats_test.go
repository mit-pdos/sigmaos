package stats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

func TestStatsd(t *testing.T) {
	const N = 1000

	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	st, err := ts.ReadSrvStats("name/")
	assert.Nil(t, err, "statsd")
	db.DPrintf(db.TEST, "st %v\n", st)
	nget := st.Counters["Nget"]

	for i := 0; i < N; i++ {
		_, err := ts.ReadSrvStats("name/")
		assert.Nil(t, err, "statsd")
	}

	st, err = ts.ReadSrvStats("name/")
	assert.Nil(t, err, "statsd")

	assert.Equal(t, nget+N+1, st.Counters["Nget"], "statsd")

	ts.Shutdown()
}
