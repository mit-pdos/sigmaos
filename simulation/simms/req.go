package simms

type Request struct {
	start uint64
	ch    chan bool
}

type Reply struct {
	req *Request
	lat uint64
}

func NewRequest(start uint64) *Request {
	return &Request{
		start: start,
		ch:    make(chan bool),
	}
}

func (r *Request) GetStart() uint64 {
	return r.start
}

func NewReply(end uint64, req *Request) *Reply {
	return &Reply{
		req: req,
		lat: end - req.start,
	}
}

func (r *Reply) GetLatency() uint64 {
	return r.lat
}
