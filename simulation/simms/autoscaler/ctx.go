package autoscaler

import (
	"fmt"
)

// Context used for debug printing
type Ctx struct {
	t     *uint64
	srvID string
}

func NewCtx(t *uint64, id string) *Ctx {
	return &Ctx{
		t:     t,
		srvID: id,
	}
}

func (c *Ctx) String() string {
	return fmt.Sprintf("[t=%v,srv=%v]", *c.t, c.srvID)
}
