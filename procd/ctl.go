package procd

import (
	"encoding/json"
	"fmt"
	"log"

	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
	"ulambda/proc"
)

const (
	RUNQLC_PRIORITY = "0"
	RUNQ_PRIORITY   = "1"
)

type CtlFile struct {
	pd *Procd
	fs.FsObj
}

func makeCtlFile(pd *Procd, uname string, parent fs.Dir) *CtlFile {
	i := inode.MakeInode(uname, 0, parent)
	return &CtlFile{pd, i}
}

func (ctl *CtlFile) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, fmt.Errorf("not supported")
}

func (ctl *CtlFile) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	p := proc.MakeEmptyProc()
	err := json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Couldn't unmarshal proc file in CtlFile.Write: %v, %v", string(b), err)
	}

	// Select which queue to put the job in
	var procPriority string
	switch p.Type {
	case proc.T_DEF:
		procPriority = RUNQ_PRIORITY
	case proc.T_LC:
		procPriority = RUNQLC_PRIORITY
	case proc.T_BE:
		procPriority = RUNQ_PRIORITY
	default:
		log.Fatalf("Error in CtlFile.Write: Unknown proc type %v", p.Type)
	}

	err = ctl.pd.runq.Put(procPriority, p.Pid, b)
	if err != nil {
		log.Fatalf("Error Put in CtlFile.Write: %v", err)
	}

	return np.Tsize(len(b)), nil
}
