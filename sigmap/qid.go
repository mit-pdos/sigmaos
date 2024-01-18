package sigmap

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

func NewQid(t Qtype, v TQversion, p Tpath) *Tqid {
	return &Tqid{Type: uint32(t), Version: uint32(v), Path: uint64(p)}
}

func NewQidPerm(perm Tperm, v TQversion, p Tpath) *Tqid {
	return NewQid(Qtype(perm>>QTYPESHIFT), v, p)
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
