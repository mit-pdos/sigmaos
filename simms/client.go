package simms

import (
	"math/rand"

	db "sigmaos/debug"
)

type Clients struct {
	mean            float64
	std             float64
	burstMultiplier float64
	toRetry         []*Request
}

func NewClients(mean, std float64) *Clients {
	return &Clients{
		mean:            mean,
		std:             std,
		burstMultiplier: 1.0,
		toRetry:         []*Request{},
	}
}

func (c *Clients) Tick(t uint64) []*Request {
	nreq := uint64((rand.NormFloat64()*c.std + c.mean) * c.burstMultiplier)
	reqs := make([]*Request, nreq)
	db.DPrintf(db.SIM_CLNT, "Generating %v requests", nreq)
	for i := range reqs {
		reqs[i] = NewRequest(t)
	}
	// Pre-pend requests to retry, and clear slice of requests to retry for next
	// tick
	reqs = append(c.toRetry, reqs...)
	c.toRetry = []*Request{}
	return reqs
}

// Retry reqs on the following tick
func (c *Clients) Retry(reqs []*Request) {
	c.toRetry = append(c.toRetry, reqs...)
}

func (c *Clients) StartBurst(multiplier float64) {
	c.burstMultiplier = multiplier
}

func (c *Clients) EndBurst() {
	c.burstMultiplier = 1.0
}
