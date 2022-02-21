package perf_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/linuxsched"
	"ulambda/perf"
)

func TestGetSamples(t *testing.T) {
	hz := perf.Hz()
	assert.NotEqual(t, 0, hz, "Hz")

	linuxsched.ScanTopology()
	// Get the cores we can run on
	m, err := linuxsched.SchedGetAffinity(os.Getpid())
	assert.Nil(t, err, "SchedGetAffinity")

	cores := map[string]bool{}
	for i := uint(0); i < linuxsched.NCores; i++ {
		if m.Test(i) {
			cores["cpu"+strconv.Itoa(int(i))] = true
		}
	}
	idle1, total1 := perf.GetCPUSample(cores)
	assert.NotEqual(t, 0, idle1, "GetCPUSample")
	assert.True(t, idle1 < total1, "total")
}
