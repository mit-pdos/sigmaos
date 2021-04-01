package schedd

import (
	"fmt"
	"log"
	"reflect"
	"time"
	"unicode"
	"unicode/utf8"

	db "ulambda/debug"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

type Dir struct {
	*Obj
}

func (sd *Sched) makeDir(path []string, t np.Tperm, p *Dir) *Dir {
	d := &Dir{}
	d.Obj = sd.MakeObj(path, t, p)
	return d
}

func (d *Dir) Create(ctx npo.CtxI, name string, perm np.Tperm, m np.Tmode) (npo.NpObj, error) {
	db.DLPrintf("SCHEDD", "%v: Create %v\n", d, name)
	// Creating a lambda is always a directory
	d1 := d.sd.makeDir(append(d.name, name), perm|np.DMDIR, d)
	d1.time = time.Now().Unix()
	d1.l = makeLambda(d.Obj.sd, name, d1)
	return d1, nil
}

func (d *Dir) Lookup(ctx npo.CtxI, p []string) ([]npo.NpObj, []string, error) {
	db.DLPrintf("SCHEDD", "%v: lookup %v %v %v\n", ctx, d, p, len(p))
	// XXX maybe include root dir
	var os []npo.NpObj
	switch len(d.name) {
	case 0:
		if len(p) == 1 && p[0] == "runq" {
			return []npo.NpObj{d.sd.runq}, nil, nil
		}
		l := d.sd.findLambda(p[0])
		if l == nil {
			return nil, nil, fmt.Errorf("not found")
		}
		d1 := l.obj
		if len(p) > 1 {
			o1 := d1.sd.makeFile(append(d1.name, p[1]), 0, d1)
			o1.time = d.time
			o1.l = l
			os = []npo.NpObj{o1}
		} else {
			os = []npo.NpObj{d1}
		}
	case 1:
		d1 := d.sd.makeDir(append(d.name, p[0]), 0, d)
		d1.time = d.time
		d1.l = d.l
		os = []npo.NpObj{d1}
	default:
		log.Fatalf("%v: Lookup: %v\n", d, p)
	}
	return os, nil, nil
}

func (d *Dir) readLambda() ([]*np.Stat, error) {
	db.DLPrintf("SCHEDD", "readLambda: %v\n", d)
	st := []*np.Stat{}

	v := reflect.ValueOf(Lambda{})
	for i := 0; i < v.NumField(); i++ {
		n := v.Type().Field(i).Name
		r, _ := utf8.DecodeRuneInString(n)
		if unicode.IsUpper(r) {
			o1 := d.sd.MakeObj(append(d.name, n), 0, d)
			o1.time = d.time
			o1.l = d.l
			st = append(st, o1.stat())
		}
	}
	return st, nil
}

func (sd *Sched) ps() []*np.Stat {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	dir := []*np.Stat{}
	for _, l := range sd.ls {
		dir = append(dir, l.obj.stat())
	}
	return dir
}

func (d *Dir) ReadDir(ctx npo.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	db.DLPrintf("SCHEDD", "readDir: %v\n", d)
	switch len(d.name) {
	case 0:
		return d.sd.ps(), nil
	case 1:
		return d.readLambda()
	default:
		log.Fatalf("ReadDir: name %v\n", d)
	}
	return nil, nil
}

func (d *Dir) WriteDir(ctx npo.CtxI, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	db.DLPrintf("SCHEDD", "%v: writeDir %v %v\n", d, off, len(data))
	switch len(d.name) {
	case 0:
		return 0, fmt.Errorf("Root is not writable %v", d)
	case 1:
		if d.l.Status == "Init" {
			err := d.l.init(data)
			if err != nil {
				return 0, nil
			}
			d.sd.spawn(d.l)
			return np.Tsize(len(data)), nil
		} else {
			return 0, fmt.Errorf("Lambda already running")
		}
	default:
		log.Fatalf("ReadDir: name %v\n", d)
	}
	return 0, nil
}
