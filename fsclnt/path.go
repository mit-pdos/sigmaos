package fsclnt

import (
	"fmt"

	np "ulambda/ninep"
	"ulambda/protclnt"
)

// The path and 9p channel associated with an fid
type Path struct {
	pc    *protclnt.ProtClnt
	cname []string
	qids  []np.Tqid
}

func makePath(pc *protclnt.ProtClnt, n []string, qs []np.Tqid) *Path {
	p := &Path{}
	p.pc = pc
	p.cname = n
	p.qids = qs
	return p
}

func (p *Path) String() string {
	str := fmt.Sprintf("{ Names %v ", p.cname)
	str += fmt.Sprintf("Qids %v }", p.qids)
	return str
}

func (p *Path) copyPath() *Path {
	qids := make([]np.Tqid, len(p.qids))
	copy(qids, p.qids)
	return makePath(p.pc, p.cname, qids)
}

func (p *Path) add(name string, q np.Tqid) {
	p.cname = append(p.cname, name)
	p.qids = append(p.qids, q)
}

// empty path = ""
func (p *Path) addn(qs []np.Tqid, path []string) {
	if len(path) == 0 {
		path = append(path, "")
	}
	for i, q := range qs {
		p.add(path[i], q)
	}
}

func (p *Path) lastqid() np.Tqid {
	return p.qids[len(p.qids)-1]
}
