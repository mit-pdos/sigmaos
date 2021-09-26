package dbd

import (
	"fmt"
	"log"
	"sync"

	"ulambda/fs"
	np "ulambda/ninep"
)

type Obj struct {
	mu   sync.Mutex
	db   *Database
	path []string
	t    np.Tperm
	id   np.Tpath
	sz   np.Tlength
	p    *Dir
}

func makeInode(string, np.Tperm, np.Tmode, fs.Dir) (fs.FsObj, error) {
	return nil, nil
}

func makeObj(db *Database, path []string, t np.Tperm, p *Dir) *Obj {
	o := &Obj{}
	id := db.nextId
	db.nextId += 1
	o.db = db
	o.t = t
	o.path = path
	o.p = p
	o.id = id
	return o
}

func (o *Obj) stat() (*np.Stat, error) {
	log.Printf("stat %v\n", o)
	st := &np.Stat{}
	if len(o.path) > 0 {
		st.Name = o.path[len(o.path)-1]
	} else {
		st.Name = "" // root
	}
	st.Mode = o.t | np.Tperm(0777)
	st.Qid = o.Qid()
	st.Uid = ""
	st.Gid = ""
	st.Length = o.sz

	return st, nil
}

func (o *Obj) SetParent(d fs.Dir) {
	// o.p = d
}

func (o *Obj) Parent() fs.Dir {
	return o.p
}

func (o *Obj) Lock() {
}

func (o *Obj) Unlock() {
}

func (o *Obj) VersionInc() {
}

func (o *Obj) SetMtime() {
}

func (o *Obj) LockAddr() *sync.Mutex {
	return nil
}

func (o *Obj) Inum() uint64 {
	return 0
}

func (o *Obj) String() string {
	s := fmt.Sprintf("p %v t %v", o.path, o.t)
	return s
}

func (o *Obj) Qid() np.Tqid {
	return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
		np.TQversion(0), o.id)
}

func (o *Obj) Perm() np.Tperm {
	return o.t
}

func (o *Obj) Size() np.Tlength {
	return o.sz
}

func (o *Obj) Version() np.TQversion {
	return 0
}

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, error) {
	return o.stat()
}

func (d *Obj) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, error) {
	return nil, nil
}

func (d *Obj) Close(ctx fs.CtxI, m np.Tmode) error {
	return nil
}

func (o *Obj) Remove(ctx fs.CtxI, name string) error {
	return nil
}

func (o *Obj) Rename(ctx fs.CtxI, from, to string) error {
	return nil
}
