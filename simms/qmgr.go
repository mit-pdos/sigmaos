package simms

type NewQMgrFn func() QMgr

type QMgr interface {
	Tick()
	Enqueue([]*Request)
	Dequeue() (*Request, bool)
	GetQLen() int
}
