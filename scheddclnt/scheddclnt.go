package scheddclnt

import (
	"fmt"
	"path"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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

func nschedd(sts []*sp.Stat) int {
	if len(sts) == 0 {
		return 0
	}
	return len(sts)
}

func MakeScheddClnt(sc *sigmaclnt.SigmaClnt, realm sp.Trealm) *ScheddClnt {
	return &ScheddClnt{0, realm, sc}
}

func (sdc *ScheddClnt) Nprocs(procdir string) (int, error) {
	sts, err := sdc.GetDir(procdir)
	if err != nil {
		return 0, nil
	}
	// Only read the proc directory if absolutely necessary.
	if db.WillBePrinted(db.SCHEDDCLNT) {
		for _, st := range sts {
			b, err := sdc.GetFile(path.Join(procdir, st.Name))
			if err != nil { // the proc may not exist anymore
				continue
			}
			p = proc.MakeEmptyProc()
			p.Unmarshal(b)
			db.DPrintf(db.SCHEDDCLNT, "%s: %v\n", procdir, p.Program)
		}
	}
	return len(sts), nil
}

func (sdc *ScheddClnt) ScheddLoad() (int, []Tload, error) {
	sts, err := sdc.GetDir(sp.SCHEDD)
	if err != nil {
		return 0, nil, err
	}
	r := nschedd(sts)
	sdloads := make([]Tload, 0, r)
	for _, st := range sts {
		sdpath := path.Join(sp.SCHEDD, st.Name, sp.RUNNING)
		nproc, err := sdc.Nprocs(path.Join(sdpath, sp.RUNNING))
		if err != nil {
			return r, nil, err
		}
		qproc, err := sdc.Nprocs(path.Join(sdpath, sp.QUEUE))
		if err != nil {
			return r, nil, err
		}
		sdloads = append(sdloads, Tload{nproc, qproc})
	}
	return r, sdloads, err
}

func (sdc *ScheddClnt) MonitorSchedds() {
	if true {
		return
	}
	var realmstr string
	if sdc.realm != "" {
		realmstr = "[" + sdc.realm.String() + "] "
	}
	go func() {
		for atomic.LoadInt32(&sdc.done) == 0 {
			n, load, err := sdc.ScheddLoad()
			if err != nil && atomic.LoadInt32(&sdc.done) == 0 {
				db.DFatalf("ScheddLoad err %v\n", err)
			}
			db.DPrintf(db.ALWAYS, "%vnschedd = %d %v\n", realmstr, n, load)
			// Sleep for 10 seconds, but do so in an interruptible way.
			for i := 0; i < 10 && atomic.LoadInt32(&sdc.done) == 0; i++ {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (sdc *ScheddClnt) Done() {
	atomic.StoreInt32(&sdc.done, 1)
}
