package schedd

import (
	"encoding/json"
	"fmt"
	"log"

	db "ulambda/debug"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

//
// File system interface to lambdas. A lambda is a directory and its
// fields are files.  An object represents either one of them.
//
type File struct {
	*Obj
}

func (sd *Sched) makeFile(path []string, t np.Tperm, p *Dir) *File {
	f := &File{}
	f.Obj = sd.MakeObj(path, t, p)
	return f
}

func (f *File) Read(ctx npo.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	db.DLPrintf("SCHEDD", "%v: Read: %v %v\n", f, off, cnt)
	if len(f.name) == 1 {
		if off != 0 {
			return nil, nil
		}
		b, err := f.sd.findRunnableLambda()
		db.DLPrintf("SCHEDD", "%v: ReadFile runnableLambda: %v %v\n", f, b, err)
		return b, err
	}
	if len(f.name) != 2 {
		log.Fatalf("Read: name != 2 %v\n", f)
	}
	if off != 0 {
		return nil, nil
	}
	f.l.mu.Lock()
	defer f.l.mu.Unlock()

	var b []byte
	switch f.name[1] {
	case "ExitStatus":
		f.l.waitForL()
		b = []byte(f.l.ExitStatus)
	case "Status":
		b = []byte(f.l.Status)
	case "Program":
		b = []byte(f.l.Program)
	case "Pid":
		b = []byte(f.l.Pid)
	case "Dir":
		b = []byte(f.l.Dir)
	case "Args":
		return json.Marshal(f.l.Args)
	case "Env":
		return json.Marshal(f.l.Env)
	case "ConsDep":
		return json.Marshal(f.l.ConsDep)
	case "ProdDep":
		return json.Marshal(f.l.ProdDep)
	case "ExitDep":
		return json.Marshal(f.l.ExitDep)
	default:
		return nil, fmt.Errorf("Unreadable field %v", f.name[0])
	}
	return b, nil
}

func (f *File) Write(ctx npo.CtxI, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	db.DLPrintf("SCHEDD", "%v: Write %v %v\n", f, off, len(data))
	if len(f.name) != 2 {
		log.Fatalf("WriteFile: name != 2 %v\n", f)
	}
	switch f.name[1] {
	case "ExitStatus":
		f.l.writeExitStatus(string(data))
	case "Status":
		f.l.writeStatus(string(data))
	case "ExitDep":
		f.l.swapExitDependency(string(data))
	default:
		return 0, fmt.Errorf("Unwritable field %v", f.name[0])
	}
	return np.Tsize(len(data)), nil
}
