package fsclnt

import (
	"fmt"

	np "ulambda/ninep"
)

type Channel struct {
	server string
	cname  []string
	qids   []np.Tqid
}

func makeChannel(s string, n []string, qs []np.Tqid) *Channel {
	c := &Channel{}
	c.server = s
	c.cname = n
	c.qids = qs
	return c
}

func (c *Channel) String() string {
	str := fmt.Sprintf("Names %v ", c.cname)
	str += fmt.Sprintf("Qids %v\n", c.qids)
	return str
}

func (c *Channel) copyChannel() *Channel {
	qids := make([]np.Tqid, len(c.qids))
	copy(qids, c.qids)
	return makeChannel(c.server, c.cname, qids)
}

func (c *Channel) add(name string, q np.Tqid) {
	c.cname = append(c.cname, name)
	c.qids = append(c.qids, q)
}

func (c *Channel) addn(path []string, qs []np.Tqid) {
	for i, s := range path {
		c.add(s, qs[i])
	}
}
