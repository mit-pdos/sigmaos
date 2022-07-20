package procdclnt

import (
	"encoding/json"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
)

type ProcdClnt struct {
	*fslib.FsLib
}

func MakeProcdClnt(fsl *fslib.FsLib) *ProcdClnt {
	return &ProcdClnt{fsl}
}

func (pdc *ProcdClnt) Nprocs(procdir string) (int, error) {
	sts, err := pdc.GetDir(procdir)
	for _, st := range sts {
		b, err := pdc.GetFile(procdir + "/" + st.Name)
		if err != nil {
			return 0, err
		}
		p := proc.MakeEmptyProc()
		err = json.Unmarshal(b, p)
		if err != nil {
			return 0, err
		}
		db.DPrintf(db.ALWAYS, "%s: %v\n", procdir, p.Program)
	}
	return len(sts), err
}

func (pdc *ProcdClnt) Nprocd() (int, []int, error) {
	sts, err := pdc.GetDir(np.PROCD)
	if err != nil {
		return 0, nil, err
	}
	r := len(sts) - 1 // -1 for ws directory
	nprocs := make([]int, 0, r)
	for _, st := range sts {
		if st.Name == "ws" {
			continue
		}
		nproc, err := pdc.Nprocs(np.PROCD + "/" + st.Name + "/running")
		if err != nil {
			return r, nil, err
		}
		nprocs = append(nprocs, nproc)
	}
	return r, nprocs, err
}
