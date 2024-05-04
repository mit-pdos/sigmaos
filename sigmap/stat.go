package sigmap

type Stat = Tstat

type Tstat struct {
	*TstatProto
}

func NewStatNull() *Tstat {
	st := &TstatProto{}
	st.Qid = NewQid(0, 0, 0)
	return &Tstat{
		TstatProto: st,
	}
}

func NewStatProto(st *TstatProto) *Tstat {
	return &Tstat{TstatProto: st}
}

func NewStat(qid *Tqid, perm Tperm, mtime uint32, name, owner string) *Tstat {
	st := &TstatProto{
		Type:   0, // XXX
		Qid:    qid,
		Mode:   uint32(perm),
		Atime:  0,
		Mtime:  mtime,
		Name:   name,
		Length: 0,
		Uid:    owner,
		Gid:    owner,
		Muid:   "",
	}
	return &Tstat{
		TstatProto: st,
	}
}

func (st *Tstat) StatProto() *TstatProto {
	return st.TstatProto
}

func (st *Tstat) Tlength() Tlength {
	return Tlength(st.Length)
}

func (st *Tstat) Tmode() Tperm {
	return Tperm(st.Mode)
}

func Names(sts []*Tstat) []string {
	r := []string{}
	for _, st := range sts {
		r = append(r, st.Name)
	}
	return r
}
