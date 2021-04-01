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

type Dir struct {
	*Obj
}

func (ld *LocalD) makeDir(path []string, t np.Tperm, p *Dir) *Dir {
	d := &Dir{}
	d.Obj = ld.MakeObj(path, t, p)
	return d
}

// Creating a lambda is always a directory
func (d *Dir) Create(ctx npo.CtxI, name string, perm np.Tperm, m np.Tmode) (npo.NpObj, error) {
	db.DLPrintf("LOCALD", "%v: Create %v\n", d, name)
	d1 := d.ld.makeDir(append(d.name, name), perm|np.DMDIR, d)
	d1.time = time.Now().Unix()
	return d1, nil
}

func (d *Dir) Lookup(ctx npo.CtxI, p []string) ([]npo.NpObj, []string, error) {
	db.DLPrintf("LOCALD", "%v: Lookup %v %v %v\n", ctx, d, p, len(p))
	// XXX maybe include root dir
	var os []npo.NpObj
	switch len(d.name) {
	case 0:
		// XXX Lookup always succeeds for now
		var d1 *Dir
		if len(p) > 1 {
			o1 := d.ld.MakeObj(append(d.name, p[1]), 0, d)
			o1.time = d.time
			os = []npo.NpObj{o1}
		} else {
			os = []npo.NpObj{d1}
		}

	case 1:
		d1 := d.ld.makeDir(append(d.name, p[0]), 0, d)
		d1.time = d.time
		d1.uid = d.uid
		os = []npo.NpObj{d1}
	default:
		log.Fatalf("%v: Lookup: %v\n", d, p)
	}
	return os, nil, nil
}

func (d *Dir) ReadDir(ctx npo.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	db.DLPrintf("LOCALD", "ReadDir: %v\n", d)
	switch len(d.name) {
	case 0:
		return []*np.Stat{}, nil
	case 1:
		// XXX
		return []*np.Stat{}, nil
	default:
		log.Fatalf("ReadDir: name %v\n", d)
	}
	return nil, nil
}

func (d *Dir) WriteDir(ctx npo.CtxI, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	db.DLPrintf("LOCALD", "%v: WriteDir %v %v\n", d, off, len(data))
	switch len(d.name) {
	case 0:
		return 0, fmt.Errorf("Root is not writable %v", d)
	case 1:
		go d.ld.spawn(data)
		return np.Tsize(len(data)), nil
	default:
		log.Fatalf("WriteDir: name %v\n", d)
	}
	return 0, fmt.Errorf("not suported")
}
