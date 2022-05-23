package procd

import (
	"encoding/json"
	"path"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
)

// Thread in charge of stealing procs.
func (pd *Procd) workStealingMonitor() {
	ticker := time.NewTicker(np.PROCD_WORK_STEAL_SCAN_TIMEOUT_MS * time.Millisecond)
	for !pd.readDone() {
		// Wait for a bit.
		<-ticker.C
		// Wait untile there is a proc to steal.
		sts, err := pd.ReadDirWatch(np.PROCD_WS, func(sts []*np.Stat) bool {
			// XXX May eventually want to ensure that none of these stealable procs
			// already belong to this procd.
			db.DPrintf("PROCD", "Found %v stealable procs", len(sts))
			return len(sts) == 0
		})
		if err != nil && np.IsErrVersion(err) {
			db.DPrintf(db.ALWAYS, "Error ReadDirWatch: %v %v", err, len(sts))
			continue
		}
		if err != nil {
			db.DFatalf("Error ReadDirWatch: %v", err)
		}
		// Wake up a thread to try to steal each proc.
		for range sts {
			pd.stealChan <- true
		}
	}
}

// Find if any procs spawned at this procd haven't been run in a while. If so,
// offer them as stealable.
func (pd *Procd) offerStealableProcs() {
	ticker := time.NewTicker(np.PROCD_STEALABLE_PROC_TIMEOUT_MS * time.Millisecond)
	for !pd.readDone() {
		// Wait for a bit.
		<-ticker.C
		runqs := []string{np.PROCD_RUNQ_LC, np.PROCD_RUNQ_BE}
		for _, runq := range runqs {
			runqPath := path.Join(np.PROCD, pd.MyAddr(), runq)
			_, err := pd.ProcessDir(runqPath, func(st *np.Stat) (bool, error) {
				// XXX Based on how we stuf Mtime into np.Stat, but this should be
				// changed, perhaps.
				if uint32(time.Now().Unix())*1000 > st.Mtime*1000+np.PROCD_STEALABLE_PROC_TIMEOUT_MS {
					db.DPrintf("PROCD", "Procd %v offering stealable proc %v", pd.MyAddr(), st.Name)
					// If proc has been haning in the runq for too long...
					target := path.Join(runqPath, st.Name) + "/"
					link := path.Join(np.PROCD_WS, st.Name)
					if err := pd.Symlink([]byte(target), link, 0777); err != nil && !np.IsErrExists(err) {
						db.DFatalf("Error Symlink: %v", err)
						return false, err
					}
				}
				return false, nil
			})
			if err != nil {
				db.DFatalf("Error ProcessDir: %v", err)
			}
		}
	}
}

func (pd *Procd) readRunqProc(procPath string) (*proc.Proc, error) {
	b, err := pd.GetFile(procPath)
	if err != nil {
		return nil, err
	}
	p := proc.MakeEmptyProc()
	err = json.Unmarshal(b, p)
	if err != nil {
		db.DFatalf("Error Unmarshal in Procd.readProc: %v", err)
		return nil, err
	}
	return p, nil
}

func (pd *Procd) claimProc(procPath string) bool {
	err := pd.Remove(procPath)
	if err != nil {
		return false
	}
	return true
}
