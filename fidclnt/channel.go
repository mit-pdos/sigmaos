package fidclnt

import (
	"fmt"

	"sigmaos/path"
	"sigmaos/protclnt"
	sp "sigmaos/sigmap"
)

// The channel associated with an fid, which connects to an object at
// a server.
type Channel struct {
	pc   *protclnt.ProtClnt
	path path.Path
	qids []*sp.Tqid
}

func newChannel(pc *protclnt.ProtClnt, path path.Path, qs []*sp.Tqid) *Channel {
	c := &Channel{}
	c.pc = pc
	c.path = path
	c.qids = qs
	return c
}

func (c *Channel) String() string {
	return fmt.Sprintf("{ Path %v Qids %v }", c.path, c.qids)
}

func (c *Channel) Path() path.Path {
	return c.path
}

func (c *Channel) SetPath(p path.Path) {
	c.path = p
}

func (c *Channel) Version() sp.TQversion {
	return c.Lastqid().Tversion()
}

func (c *Channel) Copy() *Channel {
	qids := make([]*sp.Tqid, len(c.qids))
	copy(qids, c.qids)
	return newChannel(c.pc, c.path.Copy(), qids)
}

func (c *Channel) add(name string, q *sp.TqidProto) {
	c.path = append(c.path, name)
	c.qids = append(c.qids, sp.NewTqid(q))
}

// empty path = ""
func (c *Channel) AddN(qs []*sp.TqidProto, path path.Path) {
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

func (c *Channel) Servers() *sp.Tendpoint {
	return c.pc.Servers()
}
