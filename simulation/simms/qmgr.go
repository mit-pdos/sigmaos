package simms

type NewQMgrFn func(t *uint64, ms *Microservice) QMgr

type QMgr interface {
	Tick()
	Enqueue([]*Request)
	Dequeue() (*Request, bool)
	GetQLen() int
}
