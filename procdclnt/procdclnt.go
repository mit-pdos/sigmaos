package procdclnt

import (
	//"encoding/json"
	"fmt"
	//	"path"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
	//"sigmaos/proc"
)

type ProcdClnt struct {
	done    int32
	realmid sp.Trealm
	*fslib.FsLib
}

type Tload [2]int

func (t Tload) String() string {
	return fmt.Sprintf("{r %d q %d}", t[0], t[1])
}

// -1 for ws directory
func nprocd(sts []*sp.Stat) int {
	if len(sts) == 0 {
		return 0
	}
	return len(sts) - 1
}

func MakeProcdClnt(fsl *fslib.FsLib, realmid sp.Trealm) *ProcdClnt {
	return &ProcdClnt{0, realmid, fsl}
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

// XXX fix
func (pdc *ProcdClnt) Nprocd() (int, []Tload, error) {
	return 0, make([]Tload, 0), nil
	// sts, err := pdc.GetDir(sp.PROCD)
	//
	//	if err != nil {
	//		return 0, nil, err
	//	}
	//
	// r := nprocd(sts)
	// nprocs := make([]Tload, 0, r)
	//
	//	for _, st := range sts {
	//		if st.Name == "ws" {
	//			continue
	//		}
	//		nproc, err := pdc.Nprocs(path.Join(sp.PROCD, st.Name, sp.PROCD_RUNNING))
	//		if err != nil {
	//			return r, nil, err
	//		}
	//		//		beproc, err := pdc.Nprocs(path.Join(sp.PROCD, st.Name, sp.PROCD_RUNQ_BE))
	//		beproc := 0
	//		if err != nil {
	//			return r, nil, err
	//		}
	//		//		lcproc, err := pdc.Nprocs(path.Join(sp.PROCD, st.Name, sp.PROCD_RUNQ_LC))
	//		lcproc := 0
	//		if err != nil {
	//			return r, nil, err
	//		}
	//		nprocs = append(nprocs, Tload{nproc, beproc + lcproc})
	//	}
	//
	// return r, nprocs, err
}

func (pdc *ProcdClnt) WaitProcdChange(n int) (int, error) {
	return 0, nil
	//	sts, err := pdc.ReadDirWatch(sp.PROCD, func(sts []*sp.Stat) bool {
	//		return nprocd(sts) == n
	//	})
	//
	//	if err != nil {
	//		return 0, err
	//	}
	//
	// return nprocd(sts), nil
}

func (pdc *ProcdClnt) MonitorProcds() {
	if true {
		return
	}
	var realmstr string
	if pdc.realmid != "" {
		realmstr = "[" + pdc.realmid.String() + "] "
	}
	go func() {
		for atomic.LoadInt32(&pdc.done) == 0 {
			n, load, err := pdc.Nprocd()
			if err != nil && atomic.LoadInt32(&pdc.done) == 0 {
				db.DFatalf("Nprocd err %v\n", err)
			}
			db.DPrintf(db.ALWAYS, "%vnprocd = %d %v\n", realmstr, n, load)
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
