package perf_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/perf"
)

func TestGetSamples(t *testing.T) {
	hz := perf.Hz()
	assert.NotEqual(t, 0, hz, "Hz")

	cores := perf.GetActiveCores()
	idle1, total1 := perf.GetCPUSample(cores)
	assert.NotEqual(t, 0, idle1, "GetCPUSample")
	assert.True(t, idle1 < total1, "total")
}
