package procd

import (
	//	"encoding/json"
	//	"reflect"
	//	"unicode"
	//	"unicode/utf8"
	"fmt"
	"log"
	"time"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

type Dir struct {
	*Obj
}

func (pd *Procd) makeDir(path []string, t np.Tperm, p *Dir) *Dir {
	d := &Dir{}
	d.Obj = pd.MakeObj(path, t, p)
	return d
}

// Creating a lambda is always a directory
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.NpObj, error) {
	db.DLPrintf("PROCD", "%v: Create %v\n", d, name)
	d1 := d.pd.makeDir(append(d.name, name), perm|np.DMDIR, d)
	d1.time = time.Now().Unix()
	return d1, nil
}

func (d *Dir) Lookup(ctx fs.CtxI, p []string) ([]fs.NpObj, []string, error) {
	db.DLPrintf("PROCD", "%v: Lookup %v %v %v\n", ctx, d, p, len(p))
	// XXX maybe include root dir
	var os []fs.NpObj
	switch len(d.name) {
	case 0:
		// XXX Lookup always succeeds for now
		var d1 *Dir
		if len(p) > 1 {
			o1 := d.pd.MakeObj(append(d.name, p[1]), 0, d)
			o1.time = d.time
			os = []fs.NpObj{o1}
		} else {
			os = []fs.NpObj{d1}
		}

	case 1:
		d1 := d.pd.makeDir(append(d.name, p[0]), 0, d)
		d1.time = d.time
		d1.uid = d.uid
		os = []fs.NpObj{d1}
	default:
		log.Fatalf("%v: Lookup: %v\n", d, p)
	}
	return os, nil, nil
}

func (d *Dir) ReadDir(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	db.DLPrintf("PROCD", "ReadDir: %v\n", d)
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

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	db.DLPrintf("PROCD", "%v: WriteDir %v %v\n", d, off, len(data))
	switch len(d.name) {
	case 0:
		return 0, fmt.Errorf("Root is not writable %v", d)
	case 1:
		log.Fatalf("WRITE DIR SPAWN NOT SUPPORTED")
		//		go d.pd.spawn(data)
		return np.Tsize(len(data)), nil
	default:
		log.Fatalf("WriteDir: name %v\n", d)
	}
	return 0, fmt.Errorf("not suported")
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.NpObjDir, to string) error {
	return fmt.Errorf("not supported")
}
