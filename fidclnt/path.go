package fidclnt

import (
	"fmt"

	np "ulambda/ninep"
	"ulambda/protclnt"
)

// The path and prot clnt associated with an fid
type Path struct {
	pc    *protclnt.ProtClnt
	path  []string
	qids  []np.Tqid
	uname string
}

func makePath(pc *protclnt.ProtClnt, uname string, path []string, qs []np.Tqid) *Path {
	p := &Path{}
	p.pc = pc
	p.path = path
	p.qids = qs
	p.uname = uname
	return p
}

func (p *Path) String() string {
	str := fmt.Sprintf("{ Names %v ", p.path)
	str += fmt.Sprintf("Qids %v }", p.qids)
	return str
}

func (p *Path) Uname() string {
	return p.uname
}

func (p *Path) Path() []string {
	return p.path
}

func (p *Path) Version() np.TQversion {
	return p.Lastqid().Version
}

func (p *Path) Copy() *Path {
	qids := make([]np.Tqid, len(p.qids))
	copy(qids, p.qids)
	return makePath(p.pc, p.uname, p.path, qids)
}

func (p *Path) add(name string, q np.Tqid) {
	p.path = append(p.path, name)
	p.qids = append(p.qids, q)
}

// empty path = ""
func (p *Path) AddN(qs []np.Tqid, path []string) {
	if len(path) == 0 {
		path = append(path, "")
	}
	for i, q := range qs {
		p.add(path[i], q)
	}
}

func (p *Path) Lastqid() np.Tqid {
	return p.qids[len(p.qids)-1]
}

// Simulate network partition to server that exports path
func (p *Path) Disconnect() *np.Err {
	return p.pc.Disconnect()
}

func (p *Path) Server() []string {
	return p.pc.Server()
}
