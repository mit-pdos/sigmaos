package simms

import (
	db "sigmaos/debug"
)

type Microservice struct {
	t               *uint64
	msp             *Params
	replicas        []*MicroserviceInstance
	addedReplicas   int
	removedReplicas int
	lb              LoadBalancer
}

func NewMicroservice(t *uint64, msp *Params) *Microservice {
	m := &Microservice{
		t:        t,
		msp:      msp,
		replicas: []*MicroserviceInstance{},
		lb:       NewRoundRobinLB(),
	}
	// Start off with 1 replica
	m.AddReplica()
	for _, r := range m.replicas {
		r.MarkReady()
	}
	return m
}

func (m *Microservice) AddReplica() {
	m.replicas = append(m.replicas, NewMicroserviceInstance(m.t, m.msp, m.addedReplicas, nil, nil))
	m.addedReplicas++
}

func (m *Microservice) RemoveReplica() {
	// Mark the replica "not ready"
	m.replicas[m.removedReplicas].MarkNotReady()
	m.removedReplicas++
}

func (m *Microservice) Tick(reqs []*Request) []*Reply {
	replies := []*Reply{}
	// Steer requests only to replicas which haven't been removed
	steeredReqs := m.lb.SteerRequests(reqs, m.replicas)
	steeredReqsCnt := make([]int, len(steeredReqs))
	for i, r := range steeredReqs {
		steeredReqsCnt[i] = len(r)
	}
	db.DPrintf(db.SIM_LB, "[t=%v] Steering requests to %v", *m.t, steeredReqsCnt)
	// Forward requests to replicas to which they have been steered
	for i, rs := range steeredReqs {
		replies = append(replies, m.replicas[i].Tick(rs)...)
	}
	return replies
}

type MicroserviceInstance struct {
	svc      *ServiceInstance
	memcache *Microservice
	db       *Microservice
}

func NewMicroserviceInstance(t *uint64, msp *Params, replicaID int, memcache *Microservice, db *Microservice) *MicroserviceInstance {
	return &MicroserviceInstance{
		svc:      NewServiceInstance(t, msp, replicaID),
		memcache: memcache,
		db:       db,
	}
}

func (m *MicroserviceInstance) IsReady() bool {
	return m.svc.IsReady()
}

func (m *MicroserviceInstance) MarkReady() {
	m.svc.MarkReady()
}

func (m *MicroserviceInstance) MarkNotReady() {
	m.svc.MarkNotReady()
}

func (m *MicroserviceInstance) Tick(reqs []*Request) []*Reply {
	if m.memcache != nil {
		db.DFatalf("Unimplemented: microservice with memcache")
	}
	if m.db != nil {
		db.DFatalf("Unimplemented: microservice with db")
	}
	// TODO: request type (compute vs fetch)
	// TODO: request data (fetch % chance)
	return m.svc.Tick(reqs)
}
