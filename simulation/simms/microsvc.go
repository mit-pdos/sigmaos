package simms

import (
	db "sigmaos/debug"
)

type MicroserviceParams struct {
	ID                   string
	NSlots               int    // Number of slots for processing requests in parallel
	InitTime             uint64 // Time required to initialize a new instance of the microservice
	PTime                uint64 // Time required to process each request
	KillRemovedInstances bool   // If true, kill an instance as soon as it is removed & retry any processing/queued requests
	Stateful             bool   // If true, microservice is stateful TODO: unimplemented
}

func NewMicroserviceParams(id string, nslots int, ptime uint64, initTime uint64, killRemovedInstances, stateful bool) *MicroserviceParams {
	return &MicroserviceParams{
		ID:                   id,
		NSlots:               nslots,
		InitTime:             initTime,
		PTime:                ptime,
		KillRemovedInstances: killRemovedInstances,
		Stateful:             stateful,
	}
}

type Microservice struct {
	t                *uint64
	nreqs            uint64
	msp              *MicroserviceParams
	instances        []*MicroserviceInstance
	addedInstances   int
	removedInstances int
	qmgrFn           NewQMgrFn
	lb               LoadBalancer
	autoscaler       Autoscaler
	stats            *ServiceStats
	toRetry          []*Request
}

func NewMicroservice(t *uint64, msp *MicroserviceParams, defaultOpts MicroserviceOpts, additionalOpts ...MicroserviceOpt) *Microservice {
	// Create configuration according to passed-in options
	opts := NewMicroserviceOpts(defaultOpts, additionalOpts)
	m := &Microservice{
		t:         t,
		msp:       msp,
		instances: []*MicroserviceInstance{},
		qmgrFn:    opts.NewQMgr,
		lb:        opts.NewLoadBalancer(t, opts.NewLoadBalancerMetric, opts.NewLoadBalancerSharding),
		stats:     NewServiceStats(),
		toRetry:   []*Request{},
	}
	// Start off with 1 instance
	m.AddInstance()
	for _, r := range m.instances {
		r.MarkReady()
	}
	m.autoscaler = opts.NewAutoscaler(t, m)
	return m
}

func (m *Microservice) NInstances() int {
	return m.addedInstances - m.removedInstances
}

func (m *Microservice) AddInstance() {
	m.instances = append(m.instances, NewMicroserviceInstance(m.t, m, m.msp, m.addedInstances, m.qmgrFn(m.t, m), nil, nil))
	m.addedInstances++
}

func (m *Microservice) MarkInstanceReady(idx int) {
	m.instances[idx].MarkReady()
}

func (m *Microservice) RemoveInstance() {
	// Mark the instance "not ready"
	m.instances[m.removedInstances].MarkNotReady()
	if m.msp.KillRemovedInstances {
		// "Kill" the removed instance, causing all of its processing & queued
		// requests to be retried.
		m.instances[m.removedInstances].Kill()
	}
	m.removedInstances++
}

// Retry reqs on the following tick
func (m *Microservice) Retry(reqs []*Request) {
	m.toRetry = append(m.toRetry, reqs...)
}

func (m *Microservice) Tick(reqs []*Request) []*Reply {
	m.nreqs += uint64(len(reqs))
	// Pre-pend requests to retry, and clear slice of requests to retry for next
	// tick
	reqs = append(m.toRetry, reqs...)
	m.toRetry = []*Request{}
	replies := []*Reply{}
	// Steer requests only to instances which haven't been removed
	steeredReqs := m.lb.SteerRequests(reqs, m.instances)
	steeredReqsCnt := make([]int, len(steeredReqs))
	qs := make([]Queue, len(steeredReqs))
	qlens := make([]int, len(steeredReqs))
	for i, r := range steeredReqs {
		steeredReqsCnt[i] = len(r)
		qs[i] = m.instances[i].GetQ()
		qlens[i] = m.instances[i].GetQLen()
	}
	db.DPrintf(db.SIM_LB, "[t=%v] Steering requests\n\tqlen:%v\n\treqs:%v", *m.t, qlens, steeredReqsCnt)
	// Forward requests to instances to which they have been steered
	for i, rs := range steeredReqs {
		replies = append(replies, m.instances[i].Tick(rs)...)
	}
	// Get utilization of active microservice instances
	utils := []float64{}
	for i := range steeredReqs {
		if m.instances[i].IsReady() {
			utils = append(utils, m.instances[i].GetStats().Util[*m.t])
		}
	}
	m.stats.Tick(*m.t, replies, qs, utils)
	m.autoscaler.Tick()
	return replies
}

func (m *Microservice) GetNReqs() uint64 {
	return m.nreqs
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
	stats := make([]*ServiceInstanceStats, 0, len(m.instances))
	for _, r := range m.instances {
		stats = append(stats, r.GetStats())
	}
	return stats
}

type MicroserviceInstance struct {
	svc      *ServiceInstance
	memcache *Microservice
	db       *Microservice
}

func NewMicroserviceInstance(t *uint64, ms *Microservice, msp *MicroserviceParams, instanceID int, qmgr QMgr, memcache *Microservice, db *Microservice) *MicroserviceInstance {
	return &MicroserviceInstance{
		svc:      NewServiceInstance(t, ms, msp, instanceID, qmgr),
		memcache: memcache,
		db:       db,
	}
}

func (m *MicroserviceInstance) GetStats() *ServiceInstanceStats {
	return m.svc.GetStats()
}

func (m *MicroserviceInstance) Kill() {
	m.svc.Kill()
}

func (m *MicroserviceInstance) GetQ() Queue {
	return m.svc.GetQ()
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
