package epcache_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/epcache"
	epclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SVC1  = "svc-1"
	SVC2  = "svc-2"
	IP1   = "111.111.111.111"
	IP2   = "222.222.222.222"
	IP3   = "222.222.222.222"
	PORT1 = 7777
	PORT2 = 8888
	PORT3 = 9999
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

func TestBasic(t *testing.T) {
	t1, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer t1.Shutdown()

	ts, err := newTstate(t1)
	if !assert.Nil(ts.T, err, "Err newTstate: %v", err) {
		return
	}
	defer ts.shutdown()
	db.DPrintf(db.TEST, "Started srv")
	ep1 := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddr(IP1, sp.OUTER_CONTAINER_IP, PORT1)})
	err = ts.clnt.RegisterEndpoint(SVC1, ep1)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Registered EP [%v]: %v", SVC1, ep1)
	eps, v, err := ts.clnt.GetEndpoints(SVC1, epcache.NO_VERSION)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}
	assert.NotEqual(ts.T, v, epcache.NO_VERSION, "Got back no version: %v", v)
	if !assert.Equal(ts.T, len(eps), 1, "Got back wrong num EPs: %v", len(eps)) {
		return
	}
	assert.Equal(ts.T, eps[0].String(), ep1.String(), "Got back wrong EP: %v != %v", eps[0], ep1)
	db.DPrintf(db.TEST, "Got EP [%v:%v]: %v", SVC1, v, ep1)

	// Compute the next version
	nextV := v + 1

	ch := make(chan epcache.Tversion)
	ch2 := make(chan []*sp.Tendpoint)
	// Start a goroutine to wait for the next version
	go func(v epcache.Tversion, ch chan epcache.Tversion, ch2 chan []*sp.Tendpoint) {
		db.DPrintf(db.TEST, "Get & wait for EP [%v:%v]", SVC1, nextV)
		eps, v2, err := ts.clnt.GetEndpoints(SVC1, v)
		assert.Nil(ts.T, err, "Err GetEndpoints: %v", err)
		assert.Equal(ts.T, v, v2, "Got back wrong version: %v != %v", v, v2)
		db.DPrintf(db.TEST, "Got EP after wait [%v:%v]: %v", SVC1, v2, eps)
		ch <- v2
		ch2 <- eps
	}(nextV, ch, ch2)

	// Add an EP to a different service
	ep2 := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddr(IP2, sp.OUTER_CONTAINER_IP, PORT2)})
	err = ts.clnt.RegisterEndpoint(SVC2, ep2)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Registered EP [%v]: %v", SVC2, ep2)

	select {
	case <-time.After(2 * time.Second):
	case <-ch:
		assert.False(ts.T, true, "Err Get returned early")
	}

	db.DPrintf(db.TEST, "Wait didn't return early")

	// Add another EP to the existing service
	ep3 := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddr(IP3, sp.OUTER_CONTAINER_IP, PORT3)})
	err = ts.clnt.RegisterEndpoint(SVC1, ep2)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Registered EP [%v]: %v", SVC1, ep3)

	v2 := <-ch
	assert.Equal(ts.T, nextV, v2, "Got back wrong version: %v != %v", nextV, v2)
	eps = <-ch2
	if !assert.Equal(ts.T, len(eps), 2, "Got back wrong num EPs after wait/update: %v", len(eps)) {
		return
	}
	origStrEPs := []string{ep1.String(), ep3.String()}
	strEPs := []string{eps[0].String(), eps[1].String()}
	slices.Sort(origStrEPs)
	slices.Sort(strEPs)
	assert.Equal(ts.T, len(eps), 2, "Got back wrong num EPs: %v", len(eps))
	for i := range strEPs {
		assert.Equal(ts.T, strEPs[i], origStrEPs[i], "Returned EP doesn't match: %v != %v", strEPs[i], origStrEPs[i])
	}

	err = ts.clnt.DeregisterEndpoint(SVC1, ep1)
	if !assert.Nil(ts.T, err, "Err DeregisterEndpoint: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Deregistered EP [%v]: %v", SVC1, ep1)
	eps, v, err = ts.clnt.GetEndpoints(SVC1, epcache.NO_VERSION)
	if !assert.Nil(ts.T, err, "Err GetEndpoints: %v", err) {
		return
	}
	assert.Equal(ts.T, v, nextV+1, "Got back wrong version after deregister: %v != %v", v, nextV+1)
	if !assert.Equal(ts.T, len(eps), 1, "Got back wrong num EPs: %v", len(eps)) {
		return
	}
	assert.Equal(ts.T, eps[0].String(), ep1.String(), "Got back wrong EP: %v != %v", eps[0], ep1)
	db.DPrintf(db.TEST, "Got EP [%v:%v]: %v", SVC1, v, ep1)
}
