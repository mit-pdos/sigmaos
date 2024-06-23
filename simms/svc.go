package simms

import (
	"strconv"
)

type Params struct {
	ID       string
	NSlots   int
	InitTime uint64
	PTime    uint64
	Stateful bool
}

func NewParams(id string, nslots int, ptime uint64, initTime uint64, stateful bool) *Params {
	return &Params{
		ID:       id,
		NSlots:   nslots,
		InitTime: initTime,
		PTime:    ptime,
		Stateful: stateful,
	}
}

type ServiceInstance struct {
	id              string     // ID of this service
	t               *uint64    // Number of ticks that have passed since the beginning of the simulation
	startTime       uint64     // Time at which this service instance started to initialize
	initTime        uint64     // Time required to initialize this service instance
	nslots          int        // Concurrent processing slots
	pTime           uint64     // Request processing time
	q               *Queue     // Queue of unfulfilled requests
	processing      []*Request // Slice of requests currently being processed
	processingSince []uint64   // Slice of start times at which requests began to be processed
	stateful        bool       // Indicates whether or not the service is stateful
	ready           bool       // Indicates whether or not the service is ready to accept requests
}

func NewServiceInstance(t *uint64, p *Params, replicaID int) *ServiceInstance {
	return &ServiceInstance{
		id:              p.ID + "-" + strconv.Itoa(replicaID),
		t:               t,
		startTime:       *t,
		nslots:          p.NSlots,
		pTime:           p.PTime,
		initTime:        p.InitTime,
		q:               NewQueue(),
		processing:      []*Request{},
		processingSince: []uint64{},
		stateful:        p.Stateful,
		ready:           p.InitTime == 0,
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

func (s *ServiceInstance) Tick(reqs []*Request) []*Reply {
	// If initialization time has passed, mark service ready
	if s.startTime+s.initTime >= *s.t {
		s.MarkReady()
	}
	// Enqueue new requests
	s.q.Enqueue(reqs)
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
		req, ok := s.q.Dequeue()
		if !ok {
			// Nothing left in the queue
			break
		}
		s.processing = append(s.processing, req)
		s.processingSince = append(s.processingSince, *s.t)
	}
	return reps
}
