package procdclnt

import (
	//"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	//"ulambda/proc"
)

type ProcdClnt struct {
	done int32
	*fslib.FsLib
}

type Tload [2]int

func (t Tload) String() string {
	return fmt.Sprintf("{r %d q %d}", t[0], t[1])
}

// -1 for ws directory
func nprocd(sts []*np.Stat) int {
	return len(sts) - 1
}

func MakeProcdClnt(fsl *fslib.FsLib) *ProcdClnt {
	return &ProcdClnt{0, fsl}
}

func (pdc *ProcdClnt) Nprocs(procdir string) (int, error) {
	sts, err := pdc.GetDir(procdir)
	if err != nil {
		return 0, nil
	}
	// for _, st := range sts {
	// 	b, err := pdc.GetFile(procdir + "/" + st.Name)
	// 	if err != nil { // the proc may not exist anymore
	// 		continue
	// 	}
	// 	p := proc.MakeEmptyProc()
	// 	err = json.Unmarshal(b, p)
	// 	if err != nil {
	// 		return 0, err
	// 	}
	// 	db.DPrintf("PROCDCLNT", "%s: %v\n", procdir, p.Program)
	// }
	return len(sts), nil
}

func (pdc *ProcdClnt) Nprocd() (int, []Tload, error) {
	sts, err := pdc.GetDir(np.PROCD)
	if err != nil {
		return 0, nil, err
	}
	r := nprocd(sts)
	nprocs := make([]Tload, 0, r)
	for _, st := range sts {
		if st.Name == "ws" {
			continue
		}
		nproc, err := pdc.Nprocs(np.PROCD + "/" + st.Name + "/running")
		if err != nil {
			return r, nil, err
		}
		mproc, err := pdc.Nprocs(np.PROCD + "/" + st.Name + "/runq-be")
		if err != nil {
			return r, nil, err
		}
		nprocs = append(nprocs, Tload{nproc, mproc})
	}
	return r, nprocs, err
}

func (pdc *ProcdClnt) WaitProcdChange(n int) (int, error) {
	sts, err := pdc.ReadDirWatch(np.PROCD, func(sts []*np.Stat) bool {
		return nprocd(sts) == n
	})
	if err != nil {
		return 0, err
	}
	return nprocd(sts), nil
}

func (pdc *ProcdClnt) MonitorProcds() {
	go func() {
		for atomic.LoadInt32(&pdc.done) == 0 {
			n, load, err := pdc.Nprocd()
			if err != nil && atomic.LoadInt32(&pdc.done) == 0 {
				db.DFatalf("Nprocd err %v\n", err)
			}
			db.DPrintf(db.ALWAYS, "nprocd = %d %v\n", n, load)
			// Sleep for 10 seconds, but do so in an interruptible way.
			for i := 0; i < 10 && atomic.LoadInt32(&pdc.done) == 0; i++ {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (pdc *ProcdClnt) Done() {
	atomic.StoreInt32(&pdc.done, 1)
}
