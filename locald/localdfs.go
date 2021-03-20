package locald

import (
	//	"encoding/json"
	//	"reflect"
	//	"unicode"
	//	"unicode/utf8"
	"fmt"
	"log"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

// XXX move elsewhere
type Obj struct {
	name   []string
	t      np.Tperm
	uid    uint64
	parent npo.NpObj
	ld     *LocalD
	time   int64
}

func (ld *LocalD) MakeObj(path []string, t np.Tperm, p npo.NpObj) npo.NpObj {
	o := &Obj{path, t, 0, p, ld, int64(0)}
	return o
}

func (o *Obj) Create(ctx *npo.Ctx, name string, perm np.Tperm, m np.Tmode) (npo.NpObj, error) {
	db.DLPrintf("LOCALD", "%v: Create %v\n", o, name)
	if !o.t.IsDir() {
		return nil, fmt.Errorf("not supported")
	}
	// Creating a lambda is always a directory
	o1 := o.ld.MakeObj(append(o.name, name), perm|np.DMDIR, o).(*Obj)
	o1.time = time.Now().Unix()
	return o1, nil
}

func (o *Obj) Lookup(ctx *npo.Ctx, p []string) ([]npo.NpObj, []string, error) {
	db.DLPrintf("LOCALD", "%v: Lookup %v %v %v\n", ctx, o, p, len(p))
	if !o.t.IsDir() {
		return nil, nil, fmt.Errorf("Not a directory")
	}
	// XXX maybe include root dir
	var os []npo.NpObj
	switch len(o.name) {
	case 0:
		// XXX Lookup always succeeds for now
		var o1 *Obj
		if len(p) > 1 {
			o1 := o1.ld.MakeObj(append(o1.name, p[1]), 0, o1).(*Obj)
			o1.time = o.time
		}
		os = []npo.NpObj{o1}
	case 1:
		o1 := o.ld.MakeObj(append(o.name, p[0]), 0, o).(*Obj)
		o1.time = o.time
		o1.uid = o.uid
		os = []npo.NpObj{o1}
	default:
		log.Fatalf("%v: Lookup: %v\n", o, p)
	}
	return os, nil, nil
}

// check permissions etc.
func (o *Obj) Open(ctx *npo.Ctx, m np.Tmode) error {
	return nil
}

func (o *Obj) Perm() np.Tperm {
	return o.t
}

func (o *Obj) Size() np.Tlength {
	return 0
}

func (o *Obj) Qid() np.Tqid {
	db.DLPrintf("LOCALD", "Qid %v\n", o)
	switch len(o.name) {
	case 0:
		return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(0))
	case 1, 2:
		return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(o.uid))
	default:
		log.Fatalf("Qid %v\n", o)
	}
	return np.Tqid{}
}

func (o *Obj) ReadDir(ctx *npo.Ctx, off np.Toffset, cnt np.Tsize) ([]*np.Stat, error) {
	db.DLPrintf("LOCALD", "ReadDir: %v\n", o)
	switch len(o.name) {
	case 0:
		return []*np.Stat{}, nil
	case 1:
		// XXX
		return []*np.Stat{}, nil
	default:
		log.Fatalf("ReadDir: name %v\n", o)
	}
	return nil, nil
}

func (o *Obj) ReadFile(ctx *npo.Ctx, off np.Toffset, cnt np.Tsize) ([]byte, error) {
	db.DLPrintf("LOCALD", "%v: ReadFile: %v %v\n", o, off, cnt)
	return []byte{}, fmt.Errorf("not suported")
}

func (o *Obj) Remove(ctx *npo.Ctx, name string) error {
	return fmt.Errorf("not supported")
}

func (o *Obj) Rename(ctx *npo.Ctx, from, to string) error {
	return fmt.Errorf("not supported")
}

func (o *Obj) Stat(ctx *npo.Ctx) (*np.Stat, error) {
	return o.stat(), nil
}

func (o *Obj) WriteFile(ctx *npo.Ctx, off np.Toffset, data []byte) (np.Tsize, error) {
	db.DLPrintf("LOCALD", "%v: WriteFile %v %v\n", o, off, len(data))
	return np.Tsize(len(data)), fmt.Errorf("not suported")
}

func (o *Obj) WriteDir(ctx *npo.Ctx, off np.Toffset, data []byte) (np.Tsize, error) {
	db.DLPrintf("LOCALD", "%v: WriteDir %v %v\n", o, off, len(data))
	switch len(o.name) {
	case 0:
		return 0, fmt.Errorf("Root is not writable %v", o)
	case 1:
		go o.ld.spawn(data)
		return np.Tsize(len(data)), nil
	default:
		log.Fatalf("WriteDir: name %v\n", o)
	}
	return 0, fmt.Errorf("not suported")
}

func (o *Obj) Wstat(ctx *npo.Ctx, st *np.Stat) error {
	return nil
}

func (o Obj) stat() *np.Stat {
	st := &np.Stat{}
	switch len(o.name) {
	case 0:
		st.Name = ""
	case 1:
		st.Name = o.name[0]
	case 2:
		st.Name = o.name[1]
	default:
		log.Fatalf("stat: name %v\n", o.name)
	}
	st.Mode = np.Tperm(0777) | o.t
	st.Mtime = uint32(o.time)
	st.Qid = o.Qid()
	return st
}
