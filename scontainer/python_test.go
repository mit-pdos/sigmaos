package scontainer_test

import (
	"fmt"
	"sigmaos/proc"
	"sigmaos/test"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPythonSmall(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	p := proc.NewProc("python", []string{})
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	_, err = ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)

	ts.Shutdown()
}

func TestPythonLaunch(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	p := proc.NewProc("python", []string{"/~~/pyproc/hello.py"})
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	// time.Sleep(10 * time.Second)
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	return
	_, err = ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)

	ts.Shutdown()
}
