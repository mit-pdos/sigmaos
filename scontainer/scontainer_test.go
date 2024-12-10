package scontainer_test

import (
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/docker/go-connections/nat"

	db "sigmaos/debug"
	"sigmaos/util/linux/mem"
	"sigmaos/proc"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

func TestSyscallBlock(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p := proc.NewProc("test-syscall", []string{})
	err := ts.Spawn(p)
	st, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, st.IsStatusOK(), st)
	ts.Shutdown()
}

func runMemHog(ts *test.Tstate, c chan error, id, delay, mem, dur string, nthread int) {
	p := proc.NewProc("memhog", []string{id, delay, mem, dur, strconv.Itoa(nthread)})
	if id == "LC" {
		p.SetMcpu(2000)
	}
	err := ts.Spawn(p)
	assert.Nil(ts.T, err, "Error spawn: %v", err)
	status, err := ts.WaitExit(p.GetPid())
	if err != nil {
		c <- err
		return
	}
	if !status.IsStatusOK() {
		c <- status.Error()
		return
	}
	c <- nil
}

func runMemBlock(ts *test.Tstate, mem string) *proc.Proc {
	db.DPrintf(db.TEST, "Spawning memblock for %v of memory", mem)
	p := proc.NewProc("memblock", []string{mem})
	p.SetType(proc.T_LC)
	_, errs := ts.SpawnBurst([]*proc.Proc{p}, 1)
	assert.True(ts.T, len(errs) == 0, "Error spawn: %v", errs)
	err := ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	return p
}

func evictMemBlock(ts *test.Tstate, p *proc.Proc) {
	err := ts.Evict(p.GetPid())
	assert.Nil(ts.T, err, "Error evict: %v", err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(ts.T, err, "Error WaitExit: %v", err)
	assert.True(ts.T, status.IsStatusEvicted(), "bad status: %v", status)
}

func TestLCAlone(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	mem := mem.GetTotalMem()
	lcC := make(chan error)
	go runMemHog(ts, lcC, "LC", "2s", fmt.Sprintf("%dMB", mem/2), "60s", 2)
	r1 := <-lcC
	assert.Nil(t, r1)
	ts.Shutdown()
}

func TestReapBE(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	duration := "60s"
	mem := mem.GetTotalMem()
	beC := make(chan error)
	lcC := make(chan error)
	go runMemHog(ts, lcC, "LC", "2s", fmt.Sprintf("%dMB", mem/2), duration, 2)
	go runMemHog(ts, beC, "BE", "5s", fmt.Sprintf("%dMB", (mem*3)/4), duration, 1)
	r := <-beC
	db.DPrintf(db.TEST, "beLC %v\n", r)
	r1 := <-lcC
	assert.Nil(t, r1)
	ts.Shutdown()
}

// Test that the mem blocker does indeed block off physical memory.
func TestMemBlock(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	memt := mem.GetTotalMem()
	mema := mem.GetAvailableMem()
	assert.True(ts.T, mema > memt/2, "Too little mem available")
	p := runMemBlock(ts, fmt.Sprintf("%dMB", memt*5/8))
	mema2 := mem.GetAvailableMem()
	assert.True(ts.T, mema2 < memt/2, "Too much memory available")
	evictMemBlock(ts, p)
	ts.Shutdown()
}

// Test that we can spawn a mem blocker on each node.
func TestMemBlockMany(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts.BootNode(1)
	memt := mem.GetTotalMem()
	mema := mem.GetAvailableMem()
	assert.True(ts.T, mema > memt/2, "Too little mem available")
	p1 := runMemBlock(ts, fmt.Sprintf("%dMB", memt*5/16))
	p2 := runMemBlock(ts, fmt.Sprintf("%dMB", memt*5/16))
	mema2 := mem.GetAvailableMem()
	assert.True(ts.T, mema2 < memt/2, "Too much memory available")
	evictMemBlock(ts, p1)
	evictMemBlock(ts, p2)
	ts.Shutdown()
}

func TestMemBlockManyFail(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	memt := mem.GetTotalMem()
	mema := mem.GetAvailableMem()
	assert.True(ts.T, mema > memt/2, "Too little mem available")
	p1 := runMemBlock(ts, fmt.Sprintf("%dMB", memt*5/16))
	// Give it time to start up.
	time.Sleep(5 * time.Second)
	p2 := runMemBlock(ts, fmt.Sprintf("%dMB", memt*5/16))
	evictMemBlock(ts, p1)
	status, err := ts.WaitExit(p2.GetPid())
	assert.Nil(ts.T, err, "Err waitexit: %v", err)
	assert.False(ts.T, status.IsStatusOK(), "Status ok: %v", status)
	ts.Shutdown()
}
