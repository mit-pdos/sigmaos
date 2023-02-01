package scheddclnt

import (
	//"encoding/json"
	"fmt"
	"path"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	//"sigmaos/proc"
)

type ScheddClnt struct {
	done  int32
	realm sp.Trealm
	*sigmaclnt.SigmaClnt
}

type Tload [2]int

func (t Tload) String() string {
	return fmt.Sprintf("{r %d q %d}", t[0], t[1])
}

// -1 for ws directory
func nschedd(sts []*sp.Stat) int {
	if len(sts) == 0 {
		return 0
	}
	return len(sts) - 1
}

func MakeScheddClnt(sc *sigmaclnt.SigmaClnt, realm sp.Trealm) *ScheddClnt {
	return &ScheddClnt{0, realm, sc}
}

func (pdc *ScheddClnt) Nprocs(procdir string) (int, error) {
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

func (pdc *ScheddClnt) ScheddLoad() (int, []Tload, error) {
	sts, err := pdc.GetDir(sp.SCHEDD)
	if err != nil {
		return 0, nil, err
	}
	r := nschedd(sts)
	sdloads := make([]Tload, 0, r)
	for _, st := range sts {
		sdpath := path.Join(sp.SCHEDD, st.Name, sp.RUNNING)
		nproc, err := pdc.Nprocs(path.Join(sdpath, sp.RUNNING))
		if err != nil {
			return r, nil, err
		}
		qproc, err := pdc.Nprocs(path.Join(sdpath, sp.QUEUE))
		if err != nil {
			return r, nil, err
		}
		sdloads = append(sdloads, Tload{nproc, qproc})
	}
	return r, sdloads, err
}

func (pdc *ScheddClnt) MonitorSchedds() {
	if true {
		return
	}
	var realmstr string
	if pdc.realm != "" {
		realmstr = "[" + pdc.realm.String() + "] "
	}
	go func() {
		for atomic.LoadInt32(&pdc.done) == 0 {
			n, load, err := pdc.ScheddLoad()
			if err != nil && atomic.LoadInt32(&pdc.done) == 0 {
				db.DFatalf("ScheddLoad err %v\n", err)
			}
			db.DPrintf(db.ALWAYS, "%vnschedd = %d %v\n", realmstr, n, load)
			// Sleep for 10 seconds, but do so in an interruptible way.
			for i := 0; i < 10 && atomic.LoadInt32(&pdc.done) == 0; i++ {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (pdc *ScheddClnt) Done() {
	atomic.StoreInt32(&pdc.done, 1)
}
