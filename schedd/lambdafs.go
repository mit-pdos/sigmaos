package schedd

//
//import (
//	"fmt"
//	"log"
//
//	// db "ulambda/debug"
//	np "ulambda/ninep"
//	npo "ulambda/npobjsrv"
//)
//
////
//// File system interface to lambdas. A lambda is a directory and its
//// fields are files.  An object represents either one of them.
////
//type Obj struct {
//	name   []string
//	t      np.Tperm
//	parent npo.NpObj
//	sd     *Sched
//	l      *Lambda
//	time   int64
//}
//
//func (sd *Sched) MakeObj(path []string, t np.Tperm, p npo.NpObj) *Obj {
//	o := &Obj{path, t, p, sd, nil, int64(0)}
//	return o
//}
//
//func (o *Obj) Perm() np.Tperm {
//	return o.t
//}
//
//func (o *Obj) Size() np.Tlength {
//	return 0
//}
//
//func (o *Obj) Version() np.TQversion {
//	return 0
//}
//
//func (o *Obj) Qid() np.Tqid {
//	switch len(o.name) {
//	case 0:
//		return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
//			np.TQversion(0), np.Tpath(0))
//	case 1, 2:
//		if o.name[0] == "runq" {
//			return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
//				np.TQversion(0), np.Tpath(1))
//		}
//		return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
//			np.TQversion(0), np.Tpath(o.l.uid))
//	default:
//		log.Fatalf("Qid %v\n", o)
//	}
//	return np.Tqid{}
//}
//
//// check permissions etc.
//func (o *Obj) Open(ctx npo.CtxI, m np.Tmode) error {
//	return nil
//}
//
//func (o *Obj) Close(ctx npo.CtxI, m np.Tmode) error {
//	return nil
//}
//
//func (o Obj) stat() *np.Stat {
//	st := &np.Stat{}
//	switch len(o.name) {
//	case 0:
//		st.Name = ""
//	case 1:
//		st.Name = o.name[0]
//	case 2:
//		st.Name = o.name[1]
//	default:
//		log.Fatalf("stat: name %v\n", o.name)
//	}
//	st.Mode = np.Tperm(0777) | o.t
//	st.Mtime = uint32(o.time)
//	st.Qid = o.Qid()
//	return st
//}
//
//// kill?
//func (o *Obj) Remove(ctx npo.CtxI, name string) error {
//	return fmt.Errorf("not supported")
//}
//
//func (o *Obj) Rename(ctx npo.CtxI, from, to string) error {
//	return fmt.Errorf("not supported")
//}
//
//func (o *Obj) Stat(ctx npo.CtxI) (*np.Stat, error) {
//	return o.stat(), nil
//}
//
//func (o *Obj) Wstat(ctx npo.CtxI, st *np.Stat) error {
//	return nil
//}
