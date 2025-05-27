package epcache_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/epcache"
	"sigmaos/apps/epcache/proto"
	epsrv "sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SVC1       = "svc-1"
	SVC2       = "svc-2"
	INSTANCE_1 = "instance-1"
	INSTANCE_2 = "instance-2"
	INSTANCE_3 = "instance-3"
	IP1        = "111.111.111.111"
	IP2        = "222.222.222.222"
	IP3        = "222.222.222.222"
	PORT1      = 7777
	PORT2      = 8888
	PORT3      = 9999
)

type Tstate struct {
	*test.Tstate
	j *epsrv.EPCacheJob
}

func newTstate(t1 *test.Tstate) (*Tstate, error) {
	j, err := epsrv.NewEPCacheJob(t1.SigmaClnt)
	assert.Nil(t1.T, err, "Err EPCacheJob: %v", err)
	return &Tstate{
		Tstate: t1,
		j:      j,
	}, nil
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
	defer ts.j.Stop()
	db.DPrintf(db.TEST, "Started srv")
	ep1 := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddr(IP1, sp.OUTER_CONTAINER_IP, PORT1)})
	err = ts.j.Clnt.RegisterEndpoint(SVC1, INSTANCE_1, ep1)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Registered EP [%v]: %v", SVC1, ep1)
	instances, v, err := ts.j.Clnt.GetEndpoints(SVC1, epcache.NO_VERSION)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}
	assert.NotEqual(ts.T, v, epcache.NO_VERSION, "Got back no version: %v", v)
	if !assert.Equal(ts.T, len(instances), 1, "Got back wrong num EPs: %v", len(instances)) {
		return
	}
	assert.Equal(ts.T, sp.NewEndpointFromProto(instances[0].EndpointProto).String(), ep1.String(), "Got back wrong EP: %v != %v", instances[0], ep1)
	db.DPrintf(db.TEST, "Got EP [%v:%v]: %v", SVC1, v, ep1)

	// Compute the next version
	nextV := v + 1

	ch := make(chan epcache.Tversion)
	ch2 := make(chan []*proto.Instance)
	// Start a goroutine to wait for the next version
	go func(v epcache.Tversion, ch chan epcache.Tversion, ch2 chan []*proto.Instance) {
		db.DPrintf(db.TEST, "Get & wait for EP [%v:%v]", SVC1, nextV)
		instances, v2, err := ts.j.Clnt.GetEndpoints(SVC1, v)
		assert.Nil(ts.T, err, "Err GetEndpoints: %v", err)
		assert.Equal(ts.T, v, v2, "Got back wrong version: %v != %v", v, v2)
		db.DPrintf(db.TEST, "Got EP after wait [%v:%v]: %v", SVC1, v2, instances)
		ch <- v2
		ch2 <- instances
	}(nextV, ch, ch2)

	// Add an EP to a different service
	ep2 := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddr(IP2, sp.OUTER_CONTAINER_IP, PORT2)})
	err = ts.j.Clnt.RegisterEndpoint(SVC2, INSTANCE_2, ep2)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Registered EP [%v]: %v", SVC2, ep2)

	select {
	case <-time.After(2 * time.Second):
	case <-ch:
		assert.False(ts.T, true, "Err Get returned early")
	}

	db.DPrintf(db.TEST, "Wait didn't return early 1")

	// Add another EP to the existing service
	ep3 := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{sp.NewTaddr(IP3, sp.OUTER_CONTAINER_IP, PORT3)})
	err = ts.j.Clnt.RegisterEndpoint(SVC1, INSTANCE_3, ep3)
	if !assert.Nil(ts.T, err, "Err RegisterEndpoint: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Registered EP [%v]: %v", SVC1, ep3)

	v2 := <-ch
	assert.Equal(ts.T, nextV, v2, "Got back wrong version: %v != %v", nextV, v2)
	instances = <-ch2
	if !assert.Equal(ts.T, len(instances), 2, "Got back wrong num EPs after wait/update: %v", len(instances)) {
		return
	}
	origStrEPs := []string{ep1.String(), ep3.String()}
	strEPs := []string{sp.NewEndpointFromProto(instances[0].EndpointProto).String(), sp.NewEndpointFromProto(instances[1].EndpointProto).String()}
	slices.Sort(origStrEPs)
	slices.Sort(strEPs)
	assert.Equal(ts.T, len(instances), 2, "Got back wrong num EPs: %v", len(instances))
	for i := range strEPs {
		assert.Equal(ts.T, strEPs[i], origStrEPs[i], "Returned EP doesn't match: %v != %v", strEPs[i], origStrEPs[i])
	}

	// Start a goroutine to wait for the next version
	nextV++
	go func(v epcache.Tversion, ch chan epcache.Tversion, ch2 chan []*proto.Instance) {
		db.DPrintf(db.TEST, "Get & wait for EP [%v:%v]", SVC1, nextV)
		instances, v2, err := ts.j.Clnt.GetEndpoints(SVC1, v)
		assert.Nil(ts.T, err, "Err GetEndpoints: %v", err)
		assert.Equal(ts.T, v, v2, "Got back wrong version: %v != %v", v, v2)
		db.DPrintf(db.TEST, "Got EP after wait [%v:%v]: %v", SVC1, v2, instances)
		ch <- v2
		ch2 <- instances
	}(nextV, ch, ch2)

	select {
	case <-time.After(2 * time.Second):
	case <-ch:
		assert.False(ts.T, true, "Err Get returned early")
	}

	db.DPrintf(db.TEST, "Wait didn't return early 2")

	err = ts.j.Clnt.DeregisterEndpoint(SVC1, INSTANCE_1)
	if !assert.Nil(ts.T, err, "Err DeregisterEndpoint: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Deregistered EP [%v]: %v", SVC1, ep1)

	v3 := <-ch
	assert.Equal(ts.T, nextV, v3, "Got back wrong version: %v != %v", nextV, v3)
	instances = <-ch2
	if !assert.Equal(ts.T, len(instances), 1, "Got back wrong num EPs after wait/deregister: %v", len(instances)) {
		return
	}
	assert.Equal(ts.T, sp.NewEndpointFromProto(instances[0].EndpointProto).String(), ep3.String(), "Got back wrong EP: %v != %v", instances[0], ep3)
	db.DPrintf(db.TEST, "Got EP [%v:%v]: %v", SVC1, v, ep1)
}
