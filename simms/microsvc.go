package simms

import (
	db "sigmaos/debug"
)

type MicroserviceParams struct {
	ID       string
	NSlots   int    // Number of slots for processing requests in parallel
	InitTime uint64 // Time required to initialize a new instance of the microservice
	PTime    uint64 // Time required to process each request
	Stateful bool   // If true, microservice is stateful TODO: unimplemented
}

func NewMicroserviceParams(id string, nslots int, ptime uint64, initTime uint64, stateful bool) *MicroserviceParams {
	return &MicroserviceParams{
		ID:       id,
		NSlots:   nslots,
		InitTime: initTime,
		PTime:    ptime,
		Stateful: stateful,
	}
}

type Microservice struct {
	t               *uint64
	msp             *MicroserviceParams
	replicas        []*MicroserviceInstance
	addedReplicas   int
	removedReplicas int
	lb              LoadBalancer
	stats           *ServiceStats
	autoscaler      Autoscaler
}

func NewMicroservice(t *uint64, msp *MicroserviceParams, newAutoscaler NewAutoscalerFn, newLoadBalancer NewLoadBalancerFn) *Microservice {
	m := &Microservice{
		t:        t,
		msp:      msp,
		replicas: []*MicroserviceInstance{},
		lb:       newLoadBalancer(),
		stats:    NewServiceStats(),
	}
	// Start off with 1 replica
	m.AddReplica()
	for _, r := range m.replicas {
		r.MarkReady()
	}
	m.autoscaler = newAutoscaler(t, m)
	return m
}

func (m *Microservice) NReplicas() int {
	return m.addedReplicas - m.removedReplicas
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
	m.stats.Tick(*m.t, replies)
	m.autoscaler.Tick()
	return replies
}

func (m *Microservice) GetID() string {
	return m.msp.ID
}

func (m *Microservice) GetAutoscaler() Autoscaler {
	return m.autoscaler
}

func (m *Microservice) GetServiceStats() *ServiceStats {
	return m.stats
}

func (m *Microservice) GetInstanceStats() []*ServiceInstanceStats {
	stats := make([]*ServiceInstanceStats, 0, len(m.replicas))
	for _, r := range m.replicas {
		stats = append(stats, r.GetStats())
	}
	return stats
}

type MicroserviceInstance struct {
	svc      *ServiceInstance
	memcache *Microservice
	db       *Microservice
}

func NewMicroserviceInstance(t *uint64, msp *MicroserviceParams, replicaID int, memcache *Microservice, db *Microservice) *MicroserviceInstance {
	return &MicroserviceInstance{
		svc:      NewServiceInstance(t, msp, replicaID),
		memcache: memcache,
		db:       db,
	}
}

func (m *MicroserviceInstance) GetStats() *ServiceInstanceStats {
	return m.svc.GetStats()
}

func (m *MicroserviceInstance) GetQLen() int {
	return m.svc.GetQLen()
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
