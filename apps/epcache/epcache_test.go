package epcache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	epclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	srvProc *proc.Proc
	clnt    *epclnt.EndpointCacheClnt
}

func newTstate(t1 *test.Tstate) (*Tstate, error) {
	ts := &Tstate{
		Tstate:  t1,
		srvProc: proc.NewProc("epcached", []string{}),
	}
	if err := ts.Spawn(ts.srvProc); !assert.Nil(ts.T, err, "Err Spawn: %v", err) {
		return ts, err
	}
	if err := ts.WaitStart(ts.srvProc.GetPid()); !assert.Nil(ts.T, err, "Err WaitStart: %v", err) {
		return ts, err
	}
	clnt, err := epclnt.NewEndpointCacheClnt(ts.FsLib)
	if !assert.Nil(ts.T, err, "Err NewClnt: %v", err) {
		return ts, err
	}
	ts.clnt = clnt
	return ts, nil
}

func (ts *Tstate) shutdown() {
	err := ts.Evict(ts.srvProc.GetPid())
	if assert.Nil(ts.T, err, "Err evict: %v", err) {
		return
	}
	status, err := ts.WaitExit(ts.srvProc.GetPid())
	if assert.Nil(ts.T, err, "Err evict: %v", err) {
		assert.True(ts.T, status.IsStatusEvicted(), "Unexpected exit status: %v", status)
	}
	db.DPrintf(db.TEST, "Stopped srv")
}

func TestCompile(t *testing.T) {
}

func TestRegister(t *testing.T) {
	t1, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer t1.Shutdown()

	ts, err := newTstate(t1)
	if !assert.Nil(t1.T, err, "Err newTstate: %v", err) {
		return
	}
	defer ts.shutdown()
	db.DPrintf(db.TEST, "Started srv")
}
