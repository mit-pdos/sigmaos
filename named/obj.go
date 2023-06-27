package named

import (
	"fmt"
	"hash/fnv"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

func mkTpath(pn path.Path) sessp.Tpath {
	h := fnv.New64a()
	t := time.Now() // maybe use revision
	h.Write([]byte(pn.String() + t.String()))
	return sessp.Tpath(h.Sum64())
}

// An obj is either a directory or file
type Obj struct {
	ec     *fsetcd.EtcdClnt
	pn     path.Path
	di     fsetcd.DirEntInfo
	parent sessp.Tpath
	mtime  int64
}

func makeObjDi(ec *fsetcd.EtcdClnt, pn path.Path, di fsetcd.DirEntInfo, parent sessp.Tpath) *Obj {
	o := &Obj{ec: ec, pn: pn, di: di, parent: parent}
	return o
}

func (o *Obj) String() string {
	return fmt.Sprintf("pn %q di %v parent %v", o.pn, o.di, o.parent)
}

func (o *Obj) Size() (sp.Tlength, *serr.Err) {
	return sp.Tlength(len(o.di.Nf.Data)), nil
}

func (o *Obj) SetSize(sz sp.Tlength) {
	db.DFatalf("Unimplemented")
}

func (o *Obj) Path() sessp.Tpath {
	return o.di.Path
}

func (o *Obj) Perm() sp.Tperm {
	return o.di.Perm
}

// XXX 0 should be o.parent.parent
func (o *Obj) Parent() fs.Dir {
	dir := o.pn.Dir()
	return makeDir(makeObjDi(o.ec, dir, fsetcd.DirEntInfo{Perm: sp.DMDIR | 0777, Path: o.parent}, 0))
}

// XXX SetParent

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.NAMED, "Stat: %v\n", o)
	if o, err := getObj(o.ec, o.pn, o.di.Path, o.parent); err != nil {
		return nil, err
	} else {
		st := o.stat()
		return st, nil
	}
}

func (o *Obj) stat() *sp.Stat {
	st := &sp.Stat{}
	st.Name = o.pn.Base()
	st.Qid = sp.MakeQidPerm(o.di.Perm, 0, o.di.Path)
	st.Mode = uint32(o.di.Perm)
	st.Length = uint64(len(o.di.Nf.Data))
	return st
}

func getObj(ec *fsetcd.EtcdClnt, pn path.Path, path sessp.Tpath, parent sessp.Tpath) (*Obj, *serr.Err) {
	nf, _, err := ec.GetFile(path)
	if err != nil {
		return nil, err
	}
	o := makeObjDi(ec, pn, fsetcd.DirEntInfo{Nf: nf, Perm: nf.Tperm(), Path: path}, parent)
	return o, nil
}

func (o *Obj) putObj() *serr.Err {
	nf := fsetcd.MkEtcdFile(o.di.Perm|0777, o.di.Nf.TclntId(), o.di.Nf.Data)
	return o.ec.PutFile(o.di.Path, nf)
}
