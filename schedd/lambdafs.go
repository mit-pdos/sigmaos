package schedd

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"unicode"
	"unicode/utf8"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npobjsrv"
)

//
// File system interface to lambdas. A lambda is a directory and its
// fields are files.  An object represents either one of them.
//
type Obj struct {
	name   []string
	t      np.Tperm
	parent npobjsrv.NpObj
	sd     *Sched
	l      *Lambda
}

func (sd *Sched) MakeObj(path []string, t np.Tperm, p npobjsrv.NpObj) npobjsrv.NpObj {
	o := &Obj{path, t, p, sd, nil}
	return o
}

func (o *Obj) Perm() np.Tperm {
	return o.t
}

func (o *Obj) Path() []string {
	return o.name
}

func (o *Obj) Size() np.Tlength {
	return 0
}

func (o *Obj) Qid() np.Tqid {
	if len(o.name) == 0 {
		return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(0))

	} else {
		return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(o.l.uid))
	}
}

func (o *Obj) Create(name string, perm np.Tperm, m np.Tmode) (npobjsrv.NpObj, error) {
	db.DPrintf("%v: Create %v\n", o, name)
	if !o.t.IsDir() {
		return nil, fmt.Errorf("not supported")
	}
	// Creating a lambda is always a directory
	o1 := o.sd.MakeObj(append(o.name, name), perm|np.DMDIR, o).(*Obj)
	o1.l = makeLambda(o.sd, name, o1)
	return o1, nil
}

func (o *Obj) Lookup(p []string) (npobjsrv.NpObj, error) {
	db.DPrintf("%v: lookup %v\n", o, p)
	if !o.t.IsDir() {
		return nil, fmt.Errorf("Not a directory")
	}
	if len(p) == 2 { // pid/field
		l := o.sd.findLambda(p[0])
		if l == nil {
			return nil, fmt.Errorf("not found")
		}
		db.DPrintf("%v: Lookup %v\n", o, p[1])
		o = l.obj
		p = p[1:]
	}
	if len(p) != 1 {
		log.Fatalf("%v: Lookup: %v\n", o, p)
	}
	o1 := o.sd.MakeObj(append(o.name, p[0]), 0, o).(*Obj)
	o1.l = o.l
	return o1, nil
}

func (o Obj) stat(name string) *np.Stat {
	st := &np.Stat{}
	if name == "" {
		st.Name = o.l.Pid
	} else {
		st.Name = name
	}
	st.Mode = np.Tperm(0777) | o.t
	st.Qid = o.Qid()
	st.Mtime = uint32(o.l.time)
	return st
}

func (o *Obj) readLambda() ([]*np.Stat, error) {
	db.DPrintf("readLambda: %v\n", o)
	st := []*np.Stat{}

	v := reflect.ValueOf(Lambda{})
	for i := 0; i < v.NumField(); i++ {
		n := v.Type().Field(i).Name
		r, _ := utf8.DecodeRuneInString(n)
		if unicode.IsUpper(r) {
			st = append(st, o.stat(n))
		}
	}
	return st, nil
}

func (sd *Sched) ps() []*np.Stat {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	dir := []*np.Stat{}
	for _, l := range sd.ls {
		dir = append(dir, l.obj.stat(""))
	}
	return dir
}

func (o *Obj) ReadDir() ([]*np.Stat, error) {
	db.DPrintf("readDir: %v\n", o)
	if len(o.name) == 0 { // root
		return o.sd.ps(), nil
	} else {
		return o.readLambda()
	}
}

func (o *Obj) ReadFile(off np.Toffset, cnt np.Tsize) ([]byte, error) {
	db.DPrintf("%v: ReadFile: %v %v\n", o, off, cnt)
	if len(o.name) != 2 {
		log.Fatalf("ReadFile: name != 2 %v\n", o)
	}
	if off != 0 {
		return nil, nil
	}
	o.l.mu.Lock()
	defer o.l.mu.Unlock()

	var b []byte
	switch o.name[1] {
	case "ExitStatus":
		o.l.waitForL()
		b = []byte(o.l.ExitStatus)
	case "Status":
		b = []byte(o.l.Status)
	case "Program":
		b = []byte(o.l.Program)
	case "Pid":
		b = []byte(o.l.Pid)
	case "Dir":
		b = []byte(o.l.Dir)
	case "Args":
		return json.Marshal(o.l.Args)
	case "Env":
		return json.Marshal(o.l.Env)
	case "ConsDep":
		return json.Marshal(o.l.ConsDep)
	case "ProdDep":
		return json.Marshal(o.l.ProdDep)
	case "ExitDep":
		return json.Marshal(o.l.ExitDep)
	default:
		return nil, fmt.Errorf("Unreadable field %v", o.name[0])
	}
	return b, nil
}

// kill?
func (o *Obj) Remove() error {
	return fmt.Errorf("not supported")
}

func (o *Obj) Stat() (*np.Stat, error) {
	if o.t.IsDir() {
		return o.stat(""), nil
	} else {
		return o.stat(o.name[0]), nil
	}
}

func (o *Obj) WriteFile(off np.Toffset, data []byte) (np.Tsize, error) {
	db.DPrintf("%v: writeFile %v %v\n", o, off, len(data))
	if len(o.name) != 2 {
		log.Fatalf("WriteFile: name != 2 %v\n", o)
	}
	switch o.name[1] {
	case "ExitStatus":
		o.l.writeExitStatus(string(data))
	case "Status":
		o.l.writeStatus(string(data))
	case "ExitDep":
		o.l.swapExitDependency(string(data))
	default:
		return 0, fmt.Errorf("Unwritable field %v", o.name[0])
	}
	return np.Tsize(len(data)), nil
}

func (o *Obj) WriteDir(off np.Toffset, data []byte) (np.Tsize, error) {
	db.DPrintf("%v: writeDir %v %v\n", o, off, len(data))
	if o.l.Status == "Init" {
		err := o.l.init(data)
		if err != nil {
			return 0, nil
		}
		o.sd.spawn(o.l)
		return np.Tsize(len(data)), nil
	} else {
		return 0, fmt.Errorf("Lambda already running")
	}
}

func (o *Obj) Wstat(st *np.Stat) error {
	return nil
}
