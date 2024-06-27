// The channel package has a channel for each fid.  The channel allows
// client to read/write data to the file corresponding to the fid on
// the server, clone the fid, etc.
package fidclnt

import (
	"fmt"

	"sigmaos/path"
	"sigmaos/protclnt"
	sp "sigmaos/sigmap"
)

type Channel struct {
	pc   *protclnt.ProtClnt
	pn   path.Tpathname
	qids []*sp.Tqid
}

func newChannel(pc *protclnt.ProtClnt, pn path.Tpathname, qs []*sp.Tqid) *Channel {
	c := &Channel{}
	c.pc = pc
	c.pn = pn
	c.qids = qs
	return c
}

func (c *Channel) String() string {
	return fmt.Sprintf("{ep %v pn %v Qids %v}", c.Endpoint(), c.pn, c.qids)
}

func (c *Channel) Path() path.Tpathname {
	return c.pn
}

func (c *Channel) SetPath(p path.Tpathname) {
	c.pn = p
}

func (c *Channel) Version() sp.TQversion {
	return c.Lastqid().Tversion()
}

func (c *Channel) Copy() *Channel {
	qids := make([]*sp.Tqid, len(c.qids))
	copy(qids, c.qids)
	return newChannel(c.pc, c.pn.Copy(), qids)
}

func (c *Channel) add(name string, q *sp.TqidProto) {
	c.pn = append(c.pn, name)
	c.qids = append(c.qids, sp.NewTqid(q))
}

// empty path = ""
func (c *Channel) AddN(qs []*sp.TqidProto, path path.Tpathname) {
	if len(path) == 0 {
		path = append(path, "")
	}
	for i, q := range qs {
		c.add(path[i], q)
	}
}

func (c *Channel) Lastqid() *sp.Tqid {
	return c.qids[len(c.qids)-1]
}

func (c *Channel) Qids() []*sp.Tqid {
	return c.qids
}

func (c *Channel) Endpoint() *sp.Tendpoint {
	return c.pc.Endpoints()
}
