package fidclnt

import (
	"fmt"

	"sigmaos/fcall"
	"sigmaos/protclnt"
	np "sigmaos/sigmap"
)

// The channel associated with an fid, which connects to an object at
// a server.
type Channel struct {
	pc    *protclnt.ProtClnt
	path  np.Path
	qids  []np.Tqid
	uname string
}

func makeChannel(pc *protclnt.ProtClnt, uname string, path np.Path, qs []np.Tqid) *Channel {
	c := &Channel{}
	c.pc = pc
	c.path = path
	c.qids = qs
	c.uname = uname
	return c
}

func (c *Channel) String() string {
	return fmt.Sprintf("{ Path %v Qids %v }", c.path, c.qids)
}

func (c *Channel) Uname() string {
	return c.uname
}

func (c *Channel) Path() np.Path {
	return c.path
}

func (c *Channel) SetPath(p np.Path) {
	c.path = p
}

func (c *Channel) Version() np.TQversion {
	return c.Lastqid().Version
}

func (c *Channel) Copy() *Channel {
	qids := make([]np.Tqid, len(c.qids))
	copy(qids, c.qids)
	return makeChannel(c.pc, c.uname, c.path.Copy(), qids)
}

func (c *Channel) add(name string, q np.Tqid) {
	c.path = append(c.path, name)
	c.qids = append(c.qids, q)
}

// empty path = ""
func (c *Channel) AddN(qs []np.Tqid, path np.Path) {
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
func (c *Channel) Disconnect() *fcall.Err {
	return c.pc.Disconnect()
}

func (c *Channel) Servers() []string {
	return c.pc.Servers()
}
