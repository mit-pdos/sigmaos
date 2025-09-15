package sigmap

import (
	"fmt"
	"path/filepath"
)

type Tstat struct {
	*TstatProto
}

func NewStatNull() *Tstat {
	st := &TstatProto{}
	st.Qid = NewQid(0, 0, 0).Proto()
	return &Tstat{
		TstatProto: st,
	}
}

func NewStatProto(st *TstatProto) *Tstat {
	return &Tstat{TstatProto: st}
}

func NewStat(qid Tqid, perm Tperm, mtime uint32, name, owner string) *Tstat {
	st := &TstatProto{
		Type:   0, // XXX
		Qid:    qid.Proto(),
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

func (st *Tstat) String() string {
	return fmt.Sprintf("{%v mode=%v atime=%v mtime=%v length=%v name=%v uid=%v gid=%v muid=%v}",
		st.Tqid(), st.Tmode(), st.Atime, st.Mtime, st.Tlength(), st.Name, st.Uid, st.Gid, st.Muid)
}

func (st *Tstat) Tqid() *Tqid {
	return &Tqid{*st.Qid}
}

func (st *Tstat) Tlength() Tlength {
	return Tlength(st.TstatProto.Length)
}

func (st *Tstat) LengthUint64() uint64 {
	return st.TstatProto.Length
}

func (st *Tstat) Tsize() Tsize {
	return Tsize(st.TstatProto.Length)
}

func (st *Tstat) SetLength(l Tlength) {
	st.TstatProto.Length = uint64(l)
}

func (st *Tstat) SetLengthInt(l int) {
	st.TstatProto.Length = uint64(l)
}

func (st *Tstat) Tmode() Tperm {
	return Tperm(st.TstatProto.Mode)
}

func (st *Tstat) SetMode(m Tperm) {
	st.TstatProto.Mode = uint32(m)
}

func (st *Tstat) SetMtime(t int64) {
	st.TstatProto.Mtime = uint32(t)
}

func (st *Tstat) SetUid(uid string) {
	st.TstatProto.Uid = uid
}

func (st *Tstat) SetQid(qid *Tqid) {
	st.TstatProto.Qid = qid.Proto()
}

func Names(sts []*Tstat) []string {
	r := []string{}
	for _, st := range sts {
		r = append(r, st.Name)
	}
	return r
}

func Present(sts []*Tstat, names []string) bool {
	n := 0
	m := make(map[string]bool)
	for _, n := range names {
		m[filepath.Base(n)] = true
	}
	for _, st := range sts {
		if _, ok := m[st.Name]; ok {
			n += 1
		}
	}
	return n == len(names)
}
