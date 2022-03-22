package stats_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
	"ulambda/stats"
	"ulambda/test"
)

func TestStatsd(t *testing.T) {
	ts := test.MakeTstate(t)

	st := stats.StatInfo{}
	err := ts.GetFileJson("name/"+np.STATSD, &st)
	assert.Nil(t, err, "statsd")
	nget := st.Nget

	for i := 0; i < 1000; i++ {
		_, err := ts.GetFile("name/" + np.STATSD)
		assert.Nil(t, err, "statsd")
	}

	err = ts.GetFileJson("name/"+np.STATSD, &st)
	assert.Nil(t, err, "statsd")

	assert.Equal(t, stats.Tcounter(1000)+nget+1, st.Nget, "statsd")

	last := float64(0.0)
	for i := 0; i < 5; i++ {
		err = ts.GetFileJson("name/"+np.STATSD, &st)
		assert.Nil(t, err, "statsd")
		assert.NotEqual(t, last, st.Util, "util")
		last = st.Util
		time.Sleep(100 * time.Millisecond)
	}

	ts.Shutdown()
}
