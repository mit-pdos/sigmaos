package namedv1

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"strconv"
	"strings"

	"github.com/coreos/etcd/clientv3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

// XXX remove duplication w. s3
func mkTpath(pn path.Path) sessp.Tpath {
	h := fnv.New64a()
	h.Write([]byte(pn.String()))
	return sessp.Tpath(h.Sum64())
}

func path2key(pn path.Path) string {
	d := len(pn)
	if d == 0 {
		return strconv.Itoa(d) + ":"
	}
	return strconv.Itoa(d-1) + ":" + pn.String()
}

func key2path(key string) path.Path {
	pn := key[strings.Index(key, ":"):]
	return path.Split(pn)
}

type Obj struct {
	pn      path.Path
	perm    sp.Tperm
	version sp.TQversion
	data    []byte
	mtime   int64
}

func makeObj(pn path.Path, perm sp.Tperm, v sp.TQversion, data []byte) *Obj {
	o := &Obj{}
	o.pn = pn
	o.perm = perm
	o.version = v
	o.data = data
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
	return mkTpath(o.pn)
}

func (o *Obj) Perm() sp.Tperm {
	return o.perm
}

func (o *Obj) Parent() fs.Dir {
	dir := o.pn.Dir()
	return makeDir(makeObj(dir, sp.DMDIR, 0, nil))
}

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "Stat: %v\n", o)
	if o, err := getObj(o.pn); err != nil {
		return nil, err
	} else {
		st := o.stat()
		return st, nil
	}
}

func (o *Obj) stat() *sp.Stat {
	st := &sp.Stat{}
	st.Name = o.pn.Base()
	st.Qid = sp.MakeQidPerm(o.perm, o.version, mkTpath(o.pn))
	st.Length = uint64(len(o.data))
	return st
}

func (o *Obj) putObj() *serr.Err {
	ns := &NamedState{Perm: uint32(o.perm), Data: o.data}
	if b, err := proto.Marshal(ns); err != nil {
		return serr.MkErrError(err)
	} else {
		resp, err := nd.clnt.Put(context.TODO(), path2key(o.pn), string(b))
		if err != nil {
			return serr.MkErrError(err)
		}
		db.DPrintf(db.NAMEDV1, "putObj %v\n", resp)
		return nil
	}
}

func getObj(pn path.Path) (*Obj, *serr.Err) {
	resp, err := nd.clnt.Get(context.TODO(), path2key(pn))
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "getObj %v %v\n", path2key(pn), resp)
	if len(resp.Kvs) != 1 {
		return nil, serr.MkErr(serr.TErrNotfound, pn)
	}
	data := resp.Kvs[0].Value
	ns := &NamedState{}
	if err := proto.Unmarshal(data, ns); err != nil {
		return nil, serr.MkErrError(err)
	}
	o := makeObj(pn, sp.Tperm(ns.Perm), sp.TQversion(resp.Kvs[0].Version), ns.Data)
	return o, nil
}

func mkObj(pn path.Path, perm sp.Tperm) (*Obj, *serr.Err) {
	ns := &NamedState{Perm: uint32(perm)}
	if b, err := proto.Marshal(ns); err != nil {
		return nil, serr.MkErrError(err)
	} else {
		resp, err := nd.clnt.Put(context.TODO(), path2key(pn), string(b))
		if err != nil {
			return nil, serr.MkErrError(err)
		}
		db.DPrintf(db.NAMEDV1, "putObj %v %v\n", path2key(pn), resp)
		return makeObj(pn, perm, 0, nil), nil
	}
}

func rmObj(pn path.Path) (int64, *serr.Err) {
	resp, err := nd.clnt.Delete(context.TODO(), path2key(pn))
	if err != nil {
		return 0, serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "rmObj %v %v\n", path2key(pn), resp)
	return resp.Deleted, nil
}

func readDir(pn path.Path) ([]*Obj, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "readDir %v\n", path2key(pn))
	resp, err := nd.clnt.Get(context.TODO(), path2key(pn), clientv3.WithPrefix())
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	log.Printf("resp %v\n", resp)
	ss := make([]*Obj, len(resp.Kvs))
	for i, ev := range resp.Kvs {
		ns := &NamedState{}
		if err := proto.Unmarshal(ev.Value, ns); err != nil {
			return nil, serr.MkErrError(err)
		}
		ss[i] = makeObj(key2path(string(ev.Key)), sp.Tperm(ns.Perm), sp.TQversion(resp.Kvs[0].Version), ns.Data)
	}
	return ss, nil
}
