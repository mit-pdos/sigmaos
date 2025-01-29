// The channel package has a channel for each fid.  The channel allows
// client to read/write data to the file corresponding to the fid on
// the server, clone the fid, etc.
package fidclnt

import (
	"fmt"

	sp "sigmaos/sigmap"
	spprotoclnt "sigmaos/spproto/clnt"
)

type Channel struct {
	pc   *spprotoclnt.SPProtoClnt
	qids []sp.Tqid
}

func newChannel(pc *spprotoclnt.SPProtoClnt, qs []sp.Tqid) *Channel {
	c := &Channel{}
	c.pc = pc
	c.qids = qs
	return c
}

func (c *Channel) String() string {
	return fmt.Sprintf("{ep %v Qids %v}", c.Endpoint(), c.qids)
}

func (c *Channel) Version() sp.TQversion {
	return c.Lastqid().Tversion()
}

func (c *Channel) Copy() *Channel {
	qids := make([]sp.Tqid, len(c.qids))
	copy(qids, c.qids)
	return newChannel(c.pc, qids)
}

func (c *Channel) addQid(q *sp.TqidProto) {
	c.qids = append(c.qids, sp.NewTqid(q))
}

// empty path = ""
func (c *Channel) AddQids(qs []*sp.TqidProto) {
	for _, q := range qs {
		c.addQid(q)
	}
}

func (c *Channel) Lastqid() *sp.Tqid {
	return &c.qids[len(c.qids)-1]
}

func (c *Channel) Qids() []sp.Tqid {
	return c.qids
}

func (c *Channel) Endpoint() *sp.Tendpoint {
	return c.pc.Endpoint()
}
