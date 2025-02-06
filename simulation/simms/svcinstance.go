package simms

import (
	"strconv"

	db "sigmaos/debug"
)

type ServiceInstance struct {
	id              string                // ID of this service
	t               *uint64               // Number of ticks that have passed since the beginning of the simulation
	startTime       uint64                // Time at which this service instance started to initialize
	initTime        uint64                // Time required to initialize this service instance
	nslots          int                   // Concurrent processing slots
	pTime           uint64                // Request processing time
	processing      []*Request            // Slice of requests currently being processed
	processingSince []uint64              // Slice of start times at which requests began to be processed
	stateful        bool                  // Indicates whether or not the service is stateful
	init            bool                  // Indicates whether or not the service has already initialized
	ready           bool                  // Indicates whether or not the service is ready to accept requests
	ms              *Microservice         // Microservice which this is an instance of
	qmgr            QMgr                  // Queue manager plugin
	srvStats        *ServiceInstanceStats // Stats of the current service instance
}

func NewServiceInstance(t *uint64, ms *Microservice, p *MicroserviceParams, instanceID int, qmgr QMgr) *ServiceInstance {
	return &ServiceInstance{
		id:              p.ID + "-" + strconv.Itoa(instanceID),
		t:               t,
		startTime:       *t,
		nslots:          p.NSlots,
		pTime:           p.PTime,
		initTime:        p.InitTime,
		processing:      []*Request{},
		processingSince: []uint64{},
		stateful:        p.Stateful,
		init:            p.InitTime == 0,
		ready:           p.InitTime == 0,
		ms:              ms,
		qmgr:            qmgr,
		srvStats:        NewServiceInstanceStats(t),
	}
}

func (s *ServiceInstance) MarkReady() {
	s.ready = true
}

func (s *ServiceInstance) MarkNotReady() {
	s.ready = false
}

func (s *ServiceInstance) IsReady() bool {
	return s.ready
}

func (s *ServiceInstance) GetStats() *ServiceInstanceStats {
	return s.srvStats
}

func (s *ServiceInstance) GetQ() Queue {
	return s.qmgr.GetQ()
}

func (s *ServiceInstance) GetQLen() int {
	return s.qmgr.GetQLen()
}

// Kill this instance, retrying all currently processing & queued requests at
// the client
func (s *ServiceInstance) Kill() {
	retries := make([]*Request, 0, len(s.processing)+s.qmgr.GetQLen())
	// Stop processing existing requests
	for _, r := range s.processing {
		retries = append(retries, r)
	}
	// Clear processing requests
	s.processing = []*Request{}
	s.processingSince = []uint64{}
	// Dequeue all queued requests
	for r, ok := s.qmgr.Dequeue(); ok; r, ok = s.qmgr.Dequeue() {
		retries = append(retries, r)
	}
	db.DPrintf(db.SIM_SVC, "[t=%v,svc=%v] Killed, retrying %v requests", *s.t, s.id, len(retries))
	s.ms.Retry(retries)
}

func (s *ServiceInstance) Tick(reqs []*Request) []*Reply {
	// If service had not initialized yet, and sufficient initialization time has
	// passed, mark service ready
	if !s.init && s.startTime+s.initTime <= *s.t {
		s.init = true
		s.MarkReady()
		db.DPrintf(db.SIM_SVC, "[t=%v,svc=%v] Ready", *s.t, s.id)
	}
	// Enqueue new requests
	s.qmgr.Enqueue(reqs)
	done := []int{}
	// Process existing requests
	for i := range s.processing {
		// Request done processing?
		if *s.t-s.processingSince[i] >= s.pTime {
			done = append(done, i)
		}
	}
	// Construct replies
	nDone := len(done)
	reps := make([]*Reply, nDone)
	for i := range reps {
		// Iterate over "done" backwards, since we will be removing from
		// s.processing as we go
		idx := done[nDone-1-i]
		req := s.processing[idx]
		// Remove the request which is done processing
		s.processing = append(s.processing[:idx], s.processing[idx+1:]...)
		s.processingSince = append(s.processingSince[:idx], s.processingSince[idx+1:]...)
		reps[i] = NewReply(*s.t, req)
	}
	// Dequeue queued requests
	for len(s.processing) < s.nslots {
		req, ok := s.qmgr.Dequeue()
		if !ok {
			// Nothing left in the queue
			break
		}
		s.processing = append(s.processing, req)
		s.processingSince = append(s.processingSince, *s.t)
	}
	s.qmgr.Tick()
	s.srvStats.Tick(s.IsReady(), s.processing, s.nslots, reps, s.qmgr.GetQ().GetQDelays())
	return reps
}
