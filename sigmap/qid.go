package sigmap

import (
	"fmt"
)

func (qt Qtype) String() string {
	s := ""
	if qt&QTDIR == QTDIR {
		s += "d"
	}
	if qt&QTAPPEND == QTAPPEND {
		s += "a"
	}
	if qt&QTEXCL == QTEXCL {
		s += "e"
	}
	if qt&QTMOUNT == QTMOUNT {
		s += "m"
	}
	if qt&QTAUTH == QTAUTH {
		s += "auth"
	}
	if qt&QTTMP == QTTMP {
		s += "t"
	}
	if qt&QTSYMLINK == QTSYMLINK {
		s += "s"
	}
	if s == "" {
		s = "f"
	}
	return s
}

type Tqid struct {
	TqidProto
}

func NewQid(t Qtype, v TQversion, p Tpath) Tqid {
	return Tqid{TqidProto{Type: uint32(t), Version: uint32(v), Path: uint64(p)}}
}

func NewQidPerm(perm Tperm, v TQversion, p Tpath) Tqid {
	return NewQid(Qtype(perm>>QTYPESHIFT), v, p)
}

func NewTqid(qid TqidProto) Tqid {
	return Tqid{qid}
}

func (qid Tqid) String() string {
	return fmt.Sprintf("{%v %v %v}", qid.Ttype(), qid.Tversion(), qid.Tpath())
}

func (qid Tqid) Proto() *TqidProto {
	return &qid.TqidProto
}

func NewSliceProto(qids []*Tqid) []*TqidProto {
	qp := make([]*TqidProto, len(qids))
	for i, q := range qids {
		qp[i] = q.Proto()
	}
	return qp
}

func (qid *Tqid) Tversion() TQversion {
	return TQversion(qid.Version)
}

func (qid *Tqid) Tpath() Tpath {
	return Tpath(qid.Path)
}

func (qid *Tqid) Ttype() Qtype {
	return Qtype(qid.Type)
}
