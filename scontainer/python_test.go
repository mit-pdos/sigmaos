package scontainer_test

import (
	"fmt"
	"os"
	"sigmaos/proc"
	"sigmaos/test"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPythonSmall(t *testing.T) { // TODO: modify to kill the python interpreter
	ts, _ := test.NewTstateAll(t)
	p := proc.NewPythonProc([]string{}, "ivywu")
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)
	ts.Shutdown()
}

func TestPythonLaunch(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	p := proc.NewPythonProc([]string{"/~~/pyproc/hello.py"}, "ivywu")
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)

	ts.Shutdown()
}

func TestPythonBasicImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	p := proc.NewPythonProc([]string{"/~~/pyproc/basic_import.py"}, "ivywu")
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)
	ts.Shutdown()
}

func TestPythonAWSImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	p := proc.NewPythonProc([]string{"/~~/pyproc/aws_import.py"}, "ivywu")
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)
	ts.Shutdown()
}

func TestPythonNeighborImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	p := proc.NewPythonProc([]string{"/~~/pyproc/neighbor_import/neighbor_import.py"}, "ivywu")
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)
	ts.Shutdown()
}

func TestPythonLargeImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	p := proc.NewPythonProc([]string{"/~~/pyproc/large_import.py"}, "ivywu")
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)
	return
	ts.Shutdown()
}

func TestPythonChecksumVerification(t *testing.T) {
	fmt.Printf("Starting 1st proc...\n")
	ts, _ := test.NewTstateAll(t)
	p := proc.NewPythonProc([]string{"/~~/pyproc/aws_import.py"}, "ivywu")
	start := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("cold spawn %v, start %v, exit %v\n", duration, duration2, duration3)
	ts.Shutdown()

	checksumPath := "/tmp/python/Lib/dummy_package-sigmaos-checksum"
	_, err = os.Stat(checksumPath)
	assert.Nil(t, err)

	fmt.Printf("Starting 2nd proc (cached lib)...\n")
	ts, _ = test.NewTstateAll(t)
	p2 := proc.NewPythonProc([]string{"/~~/pyproc/aws_import.py"}, "ivywu")
	start2 := time.Now()
	err = ts.Spawn(p2)
	assert.Nil(ts.T, err)
	duration4 := time.Since(start2)
	err = ts.WaitStart(p2.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration5 := time.Since(start2)
	status, err = ts.WaitExit(p2.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration6 := time.Since(start2)
	fmt.Printf("warm spawn %v, start %v, exit %v\n", duration4, duration5, duration6)
	ts.Shutdown()

	_, err = os.Stat(checksumPath)
	assert.Nil(t, err)
	err = os.Remove(checksumPath)
	assert.Nil(t, err)
	_, err = os.Stat(checksumPath)
	assert.NotNil(t, err)

	fmt.Printf("Starting 3rd proc (invalid cache)...\n")
	ts, _ = test.NewTstateAll(t)
	p3 := proc.NewPythonProc([]string{"/~~/pyproc/aws_import.py"}, "ivywu")
	start3 := time.Now()
	err = ts.Spawn(p3)
	assert.Nil(ts.T, err)
	duration7 := time.Since(start3)
	err = ts.WaitStart(p3.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration8 := time.Since(start3)
	status, err = ts.WaitExit(p3.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration9 := time.Since(start3)
	fmt.Printf("warm spawn %v, start %v, exit %v\n", duration7, duration8, duration9)
	ts.Shutdown()
}
