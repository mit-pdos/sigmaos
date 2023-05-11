package stats_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
	"sigmaos/stats"
	"sigmaos/test"
)

func TestStatsd(t *testing.T) {
	ts := test.MakeTstate(t)

	st := &stats.Stats{}
	err := ts.GetFileJson("name/"+sp.STATSD, st)
	assert.Nil(t, err, "statsd")
	log.Printf("st %v\n", st)
	nget := st.Nget

	for i := 0; i < 1000; i++ {
		_, err := ts.GetFile("name/" + sp.STATSD)
		assert.Nil(t, err, "statsd")
	}

	err = ts.GetFileJson("name/"+sp.STATSD, st)
	assert.Nil(t, err, "statsd")

	assert.Equal(t, stats.Tcounter(1000)+nget+1, st.Nget, "statsd")

	ts.Shutdown()
}
