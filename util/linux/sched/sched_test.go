package sched_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	linuxsched "sigmaos/util/linux/sched"
)

func TestCompile(t *testing.T) {
}

func TestBasic(t *testing.T) {
	pid := os.Getpid()
	// Get the cores we can run on
	m, err := linuxsched.SchedGetAffinity(pid)
	assert.Nil(t, err, "SchedGetAffinity")
	core := false
	for i := uint(0); i < linuxsched.GetNCores(); i++ {
		if m.Test(i) {
			core = true
		}
	}
	assert.True(t, core, "Nnumber of cores")
}
