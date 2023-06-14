package named

import (
	"fmt"
	"hash/fnv"
	"time"

	"google.golang.org/protobuf/proto"

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
	ec      *etcdclnt.EtcdClnt
	pn      path.Path
	path    sessp.Tpath
	perm    sp.Tperm
	version sp.TQversion
	parent  sessp.Tpath
	data    []byte
	mtime   int64
}

func makeObj(ec *etcdclnt.EtcdClnt, pn path.Path, perm sp.Tperm, v sp.TQversion, p sessp.Tpath, parent sessp.Tpath, data []byte) *Obj {
	o := &Obj{ec: ec, pn: pn, perm: perm, version: v, path: p, data: data, parent: parent}
	return o
}

func (o *Obj) String() string {
	return fmt.Sprintf("pn '%v' perm %v v %v %d", o.pn, o.perm, o.version, len(o.data))
}

func (o *Obj) Size() (sp.Tlength, *serr.Err) {
	return sp.Tlength(len(o.data)), nil
}

func (o *Obj) SetSize(sz sp.Tlength) {
	db.DFatalf("Unimplemented")
}

func (o *Obj) Path() sessp.Tpath {
	return o.path
}

func (o *Obj) Perm() sp.Tperm {
	return o.perm
}

// XXX 0 should be o.parent.parent
func (o *Obj) Parent() fs.Dir {
	dir := o.pn.Dir()
	return makeDir(makeObj(o.ec, dir, sp.DMDIR|0777, 0, o.parent, 0, nil))
}

// XXX SetParent

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.NAMED, "Stat: %v\n", o)
	if o, err := getObj(o.ec, o.pn, o.path, o.parent); err != nil {
		return nil, err
	} else {
		st := o.stat()
		return st, nil
	}
}

func (o *Obj) stat() *sp.Stat {
	st := &sp.Stat{}
	st.Name = o.pn.Base()
	st.Qid = sp.MakeQidPerm(o.perm, o.version, o.path)
	st.Mode = uint32(o.perm)
	st.Length = uint64(len(o.data))
	return st
}

func getObj(ec *etcdclnt.EtcdClnt, pn path.Path, path sessp.Tpath, parent sessp.Tpath) (*Obj, *serr.Err) {
	nf, v, err := ec.GetFile(path)
	if err != nil {
		return nil, err
	}
	o := makeObj(ec, pn, sp.Tperm(nf.Perm), sp.TQversion(v), path, parent, nf.Data)
	return o, nil
}

// Marshal empty file or directory
func marshalObj(perm sp.Tperm, path sessp.Tpath) (*etcdclnt.NamedFile, *serr.Err) {
	var fdata []byte
	if perm.IsDir() {
		nd := &etcdclnt.NamedDir{}
		nd.Ents = append(nd.Ents, &etcdclnt.DirEnt{Name: ".", Path: uint64(path)})
		d, err := proto.Marshal(nd)
		if err != nil {
			return nil, serr.MkErrError(err)
		}
		fdata = d
	}
	return &etcdclnt.NamedFile{Perm: uint32(perm | 0777), Data: fdata}, nil
}

func (o *Obj) putObj() *serr.Err {
	nf := &etcdclnt.NamedFile{Perm: uint32(o.perm), Data: o.data}
	return o.ec.PutFile(o.path, nf)
}
