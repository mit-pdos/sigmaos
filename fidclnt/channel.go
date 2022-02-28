package fidclnt

import (
	"fmt"

	np "ulambda/ninep"
	"ulambda/protclnt"
)

// The channel associated with an fid, which connects to an object at
// a server.
type Channel struct {
	pc    *protclnt.ProtClnt
	path  []string
	qids  []np.Tqid
	uname string
}

func makeChannel(pc *protclnt.ProtClnt, uname string, path []string, qs []np.Tqid) *Channel {
	c := &Channel{}
	c.pc = pc
	c.path = path
	c.qids = qs
	c.uname = uname
	return c
}

func (c *Channel) String() string {
	str := fmt.Sprintf("{ Path %v ", c.path)
	str += fmt.Sprintf("Qids %v }", c.qids)
	return str
}

func (c *Channel) Uname() string {
	return c.uname
}

func (c *Channel) Path() []string {
	return c.path
}

func (c *Channel) Version() np.TQversion {
	return c.Lastqid().Version
}

func (c *Channel) Copy() *Channel {
	qids := make([]np.Tqid, len(c.qids))
	copy(qids, c.qids)
	return makeChannel(c.pc, c.uname, c.path, qids)
}

func (c *Channel) add(name string, q np.Tqid) {
	c.path = append(c.path, name)
	c.qids = append(c.qids, q)
}

// empty path = ""
func (c *Channel) AddN(qs []np.Tqid, path []string) {
	if len(path) == 0 {
		path = append(path, "")
	}
	for i, q := range qs {
		c.add(path[i], q)
	}
}

func (c *Channel) Lastqid() np.Tqid {
	return c.qids[len(c.qids)-1]
}

// Simulate network partition to server that exports path
func (c *Channel) Disconnect() *np.Err {
	return c.pc.Disconnect()
}

func (c *Channel) Server() []string {
	return c.pc.Server()
}
