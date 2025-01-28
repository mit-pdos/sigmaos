package simms

type NewQMgrFn func(t *uint64, ms *Microservice) QMgr

type QMgr interface {
	Tick()
	Enqueue([]*Request)
	Dequeue() (*Request, bool)
	GetQ() Queue
	GetQLen() int
}

type Queue interface {
	Enqueue([]*Request)
	Dequeue() (*Request, bool)
	TimeoutReqs(timeout uint64) []*Request
	GetLen() int
}
