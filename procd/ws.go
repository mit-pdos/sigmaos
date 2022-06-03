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
	ticker := time.NewTicker(np.Conf.Procd.WORK_STEAL_SCAN_TIMEOUT)
	for !pd.readDone() {
		// Wait for a bit.
		<-ticker.C
		var nStealable int
		// Wait untile there is a proc to steal.
		sts, err := pd.ReadDirWatch(np.PROCD_WS, func(sts []*np.Stat) bool {
			nStealable = len(sts)
			// Discount procs already on this procd
			for _, st := range sts {
				if _, ok := pd.getProcStatus(proc.Tpid(st.Name)); ok {
					nStealable--
				}
			}
			db.DPrintf("PROCD", "Found %v stealable procs, of which %v belonged to other procds", len(sts), nStealable)
			return nStealable == 0
		})
		if err != nil && np.IsErrVersion(err) {
			db.DPrintf("PROCD_ERR", "Error ReadDirWatch: %v %v", err, len(sts))
			continue
		}
		if err != nil {
			pd.perf.Done()
			db.DFatalf("Error ReadDirWatch: %v", err)
		}
		// Wake up a thread to try to steal each proc.
		for i := 0; i < nStealable; i++ {
			pd.stealChan <- true
		}
	}
}

// Find if any procs spawned at this procd haven't been run in a while. If so,
// offer them as stealable.
func (pd *Procd) offerStealableProcs() {
	ticker := time.NewTicker(np.Conf.Procd.STEALABLE_PROC_TIMEOUT)
	for !pd.readDone() {
		// Wait for a bit.
		<-ticker.C
		runqs := []string{np.PROCD_RUNQ_LC, np.PROCD_RUNQ_BE}
		for _, runq := range runqs {
			runqPath := path.Join(np.PROCD, pd.MyAddr(), runq)
			_, err := pd.ProcessDir(runqPath, func(st *np.Stat) (bool, error) {
				// XXX Based on how we stuf Mtime into np.Stat (at a second
				// granularity), but this should be changed, perhaps.
				if uint32(time.Now().Unix())*1000 > st.Mtime*1000+uint32(np.Conf.Procd.STEALABLE_PROC_TIMEOUT/time.Millisecond) {
					db.DPrintf("PROCD", "Procd %v offering stealable proc %v", pd.MyAddr(), st.Name)
					// If proc has been haning in the runq for too long...
					target := path.Join(runqPath, st.Name) + "/"
					link := path.Join(np.PROCD_WS, st.Name) + "-SYMLINK"
					if err := pd.Symlink([]byte(target), link, 0777|np.DMTMP); err != nil && !np.IsErrExists(err) {
						db.DFatalf("Error Symlink: %v", err)
						return false, err
					}
				}
				return false, nil
			})
			if err != nil {
				pd.perf.Done()
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
		pd.perf.Done()
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
