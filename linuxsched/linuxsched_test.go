package linuxsched_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/linuxsched"
)

func TestBasic(t *testing.T) {
	pid := os.Getpid()
	// Get the cores we can run on
	m, err := linuxsched.SchedGetAffinity(pid)
	assert.Nil(t, err, "SchedGetAffinity")
	core := false
	for i := uint(0); i < linuxsched.NCores; i++ {
		if m.Test(i) {
			core = true
		}
	}
	assert.True(t, core, "Ncore")
}
