package stats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
	"sigmaos/stats"
	"sigmaos/test"
)

func TestStatsd(t *testing.T) {
	ts := test.MakeTstate(t)

	st := stats.StatInfo{}
	err := ts.GetFileJson("name/"+sp.STATSD, &st)
	assert.Nil(t, err, "statsd")
	nget := st.StatsCopy().Nget

	for i := 0; i < 1000; i++ {
		_, err := ts.GetFile("name/" + sp.STATSD)
		assert.Nil(t, err, "statsd")
	}

	err = ts.GetFileJson("name/"+sp.STATSD, &st)
	assert.Nil(t, err, "statsd")

	assert.Equal(t, stats.Tcounter(1000)+nget+1, st.StatsCopy().Nget, "statsd")

	ts.Shutdown()
}
