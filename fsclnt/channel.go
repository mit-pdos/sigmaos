package fsclnt

import (
	"fmt"

	np "ulambda/ninep"
	"ulambda/npclnt"
)

type Channel struct {
	npch  *npclnt.NpChan
	cname []string
	qids  []np.Tqid
}

func makeChannel(npc *npclnt.NpChan, n []string, qs []np.Tqid) *Channel {
	c := &Channel{}
	c.npch = npc
	c.cname = n
	c.qids = qs
	return c
}

func (c *Channel) String() string {
	str := fmt.Sprintf("{ Names %v ", c.cname)
	str += fmt.Sprintf("Qids %v }", c.qids)
	return str
}

func (c *Channel) copyChannel() *Channel {
	qids := make([]np.Tqid, len(c.qids))
	copy(qids, c.qids)
	return makeChannel(c.npch, c.cname, qids)
}

func (c *Channel) add(name string, q np.Tqid) {
	c.cname = append(c.cname, name)
	c.qids = append(c.qids, q)
}

// empty path = ""
func (c *Channel) addn(qs []np.Tqid, path []string) {
	if len(path) == 0 {
		path = append(path, "")
	}
	for i, q := range qs {
		c.add(path[i], q)
	}
}

func (c *Channel) lastqid() np.Tqid {
	return c.qids[len(c.qids)-1]
}
