package namedv1

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/etcdclnt"
	"sigmaos/fs"
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

func path2key(path sessp.Tpath) string {
	return strconv.FormatUint(uint64(path), 16)
}

func key2path(key string) sessp.Tpath {
	p, err := strconv.ParseUint(key, 16, 64)
	if err != nil {
		db.DFatalf("ParseUint %v err %v\n", key, err)
	}
	return sessp.Tpath(p)
}

type Obj struct {
	pn      path.Path
	path    sessp.Tpath
	perm    sp.Tperm
	version sp.TQversion
	parent  sessp.Tpath
	data    []byte
	mtime   int64
}

func makeObj(pn path.Path, perm sp.Tperm, v sp.TQversion, p sessp.Tpath, parent sessp.Tpath, data []byte) *Obj {
	o := &Obj{}
	o.pn = pn
	o.perm = perm
	o.version = v
	o.path = p
	o.data = data
	o.parent = parent
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
	return makeDir(makeObj(dir, sp.DMDIR|0777, 0, o.parent, 0, nil))
}

// XXX SetParent

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "Stat: %v\n", o)
	if o, err := getObj(o.pn, o.path, o.parent); err != nil {
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

func getObj(pn path.Path, path sessp.Tpath, parent sessp.Tpath) (*Obj, *serr.Err) {
	nf, v, err := etcdclnt.GetFile(nd.clnt, path)
	if err != nil {
		return nil, err
	}
	o := makeObj(pn, sp.Tperm(nf.Perm), sp.TQversion(v), path, parent, nf.Data)
	return o, nil
}

func mkRootDir() *serr.Err {
	b, r := marshalObj(sp.DMDIR, etcdclnt.ROOT)
	if r != nil {
		return r
	}
	resp, err := nd.clnt.Put(context.TODO(), path2key(etcdclnt.ROOT), string(b))
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "mkRoot: putObj %v\n", resp)
	return nil
}

// Marshal empty file or directory
func marshalObj(perm sp.Tperm, path sessp.Tpath) ([]byte, *serr.Err) {
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
	nf := &etcdclnt.NamedFile{Perm: uint32(perm | 0777), Data: fdata}
	b, err := proto.Marshal(nf)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

// XXX retry
func addObj(pn path.Path, dp sessp.Tpath, dir *etcdclnt.NamedDir, dperm sp.Tperm, v sp.TQversion, p sessp.Tpath, perm sp.Tperm) (*Obj, *serr.Err) {
	b, r := marshalObj(perm, p)
	if r != nil {
		return nil, r
	}
	d1, r := etcdclnt.MarshalDir(dir, dperm)
	if r != nil {
		return nil, r
	}
	// Update directory if new file/dir doesn't exist and directory
	// hasn't changed.
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.Version(path2key(p)), "=", 0),
		clientv3.Compare(clientv3.Version(path2key(dp)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpPut(path2key(p), string(b)),
		clientv3.OpPut(path2key(dp), string(d1))}
	resp, err := nd.clnt.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "addObj %v %v\n", p, resp)
	if !resp.Succeeded {
		return nil, serr.MkErr(serr.TErrExists, p)
	}
	return makeObj(pn, perm, 0, p, dp, nil), nil
}

func rmObj(d sessp.Tpath, dir *etcdclnt.NamedDir, dperm sp.Tperm, v sp.TQversion, del sessp.Tpath) *serr.Err {
	d1, r := etcdclnt.MarshalDir(dir, dperm)
	if r != nil {
		return r
	}
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.Version(path2key(del)), ">", 0),
		clientv3.Compare(clientv3.Version(path2key(d)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpDelete(path2key(del)),
		clientv3.OpPut(path2key(d), string(d1))}
	resp, err := nd.clnt.Txn(context.TODO()).
		If(cmp...).Then(ops...).Commit()
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "rmObj %v %v\n", del, resp)
	if !resp.Succeeded {
		return serr.MkErr(serr.TErrNotfound, del)
	}
	return nil
}

// XXX retry
func mvObj(d sessp.Tpath, dir *etcdclnt.NamedDir, dperm sp.Tperm, v sp.TQversion, del sessp.Tpath) *serr.Err {
	d1, r := etcdclnt.MarshalDir(dir, dperm)
	if r != nil {
		return r
	}
	var cmp []clientv3.Cmp
	var ops []clientv3.Op
	if del != 0 {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(path2key(del)), ">", 0),
			clientv3.Compare(clientv3.Version(path2key(d)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpDelete(path2key(del)),
			clientv3.OpPut(path2key(d), string(d1))}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(path2key(d)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpPut(path2key(d), string(d1))}
	}
	resp, err := nd.clnt.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "mvObj %v %v\n", d, resp)
	if !resp.Succeeded {
		return serr.MkErr(serr.TErrNotfound, d)
	}
	return nil
}

// XXX retry
func mvObjat(df sessp.Tpath, dirf *etcdclnt.NamedDir, dirfperm sp.Tperm, vf sp.TQversion, dt sessp.Tpath, dirt *etcdclnt.NamedDir, dirtperm sp.Tperm, vt sp.TQversion, del sessp.Tpath) *serr.Err {
	bf, r := etcdclnt.MarshalDir(dirf, dirfperm)
	if r != nil {
		return r
	}
	bt, r := etcdclnt.MarshalDir(dirt, dirtperm)
	if r != nil {
		return r
	}
	var cmp []clientv3.Cmp
	var ops []clientv3.Op
	if del != 0 {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(path2key(del)), ">", 0),
			clientv3.Compare(clientv3.Version(path2key(df)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(path2key(dt)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpDelete(path2key(del)),
			clientv3.OpPut(path2key(df), string(bf)),
			clientv3.OpPut(path2key(dt), string(bt)),
		}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(path2key(df)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(path2key(dt)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpPut(path2key(df), string(bf)),
			clientv3.OpPut(path2key(dt), string(bt)),
		}
	}
	resp, err := nd.clnt.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "mvObjAt %v %v\n", del, resp)
	if !resp.Succeeded {
		return serr.MkErr(serr.TErrNotfound, del)
	}
	return nil
}

func (o *Obj) putObj() *serr.Err {
	nf := &etcdclnt.NamedFile{Perm: uint32(o.perm), Data: o.data}
	return etcdclnt.PutFile(nd.clnt, o.path, nf)
}
