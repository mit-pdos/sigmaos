package procd

import (
	"path"
	"strings"
	"time"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/semclnt"
)

// Thread in charge of stealing procs.
func (pd *Procd) startWorkStealingMonitors() {
	go pd.monitorWSQueue(path.Join(np.PROCD_WS, np.PROCD_RUNQ_LC))
	go pd.monitorWSQueue(path.Join(np.PROCD_WS, np.PROCD_RUNQ_BE))
}

// Monitor a Work-Stealing queue.
func (pd *Procd) monitorWSQueue(wsQueue string) {
	ticker := time.NewTicker(np.Conf.Procd.WORK_STEAL_SCAN_TIMEOUT)
	for !pd.readDone() {
		// Wait for a bit to avoid overwhelming named.
		<-ticker.C

		var nStealable int
		// Wait untile there is a proc to steal.
		sts, err := pd.ReadDirWatch(wsQueue, func(sts []*np.Stat) bool {
			// Any procs are local?
			anyLocal := false
			nStealable = len(sts)
			// Discount procs already on this procd
			for _, st := range sts {
				// See if this proc was spawned on this procd or has been stolen. If
				// so, discount it from the count of stealable procs.
				b, err := pd.GetFile(path.Join(wsQueue, st.Name))
				if err != nil || strings.Contains(string(b), pd.MyAddr()) {
					anyLocal = true
					nStealable--
				}
			}
			// If any procs are local (possibly BE procs which weren't spawned before
			// due to rate-limiting), try to spawn one of them, so that we don't
			// deadlock with all the workers sleeping & BE procs waiting to be
			// spawned.
			if anyLocal {
				nStealable++
			}
			db.DPrintf("PROCD", "Found %v stealable procs, of which %v belonged to other procds", len(sts), nStealable)
			return nStealable == 0
		})
		// Version error may occur if another procd has modified the ws dir, and
		// unreachable err may occur if the other procd is shutting down.
		if err != nil && (np.IsErrVersion(err) || np.IsErrUnreachable(err)) {
			db.DPrintf("PROCD_ERR", "Error ReadDirWatch: %v %v", err, len(sts))
			db.DPrintf(db.ALWAYS, "Error ReadDirWatch: %v %v", err, len(sts))
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
					link := path.Join(np.PROCD_WS, runq, st.Name)
					if err := pd.Symlink([]byte(target), link, 0777|np.DMTMP); err != nil && !np.IsErrExists(err) {
						db.DFatalf("Error Symlink: %v", err)
						return false, err
					}
				}
				return false, nil
			})
			if err != nil {
				pd.perf.Done()
				db.DFatalf("Error ProcessDir: p %v err %v myIP %v", runqPath, err, pd.MyAddr())
			}
		}
	}
}

// Delete the work-stealing symlink for a proc.
func (pd *Procd) deleteWSSymlink(st *np.Stat, procPath string, p *LinuxProc, isRemote bool) {
	// If this proc is remote, remove the symlink.
	if isRemote {
		// Remove the symlink (don't follow).
		pd.Remove(procPath[:len(procPath)-1])
	} else {
		// If proc was offered up for work stealing...
		if uint32(time.Now().Unix())*1000 > st.Mtime*1000+uint32(np.Conf.Procd.STEALABLE_PROC_TIMEOUT/time.Millisecond) {
			var runq string
			if p.attr.Type == proc.T_LC {
				runq = np.PROCD_RUNQ_LC
			} else {
				runq = np.PROCD_RUNQ_BE
			}
			link := path.Join(np.PROCD_WS, runq, st.Name)
			pd.Remove(link)
		}
	}
}

func (pd *Procd) readRunqProc(procPath string) (*proc.Proc, error) {
	p := proc.MakeEmptyProc()
	err := pd.GetFileJson(procPath, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (pd *Procd) claimProc(p *proc.Proc, procPath string) bool {
	// Create an ephemeral semaphore for the parent proc to wait on. We do this
	// optimistically, since it must already be there when we actually do the
	// claiming.
	semStart := semclnt.MakeSemClnt(pd.FsLib, path.Join(p.ParentDir, proc.START_SEM))
	err1 := semStart.Init(np.DMTMP)
	// If someone beat us to the semaphore creation, we can't have possibly
	// claimed the proc, so bail out. If the procd that created the semaphore
	// crashed, its semaphore will be automatically removed (since the semaphore
	// is ephemeral) and another procd will eventually re-try the claim.
	if err1 != nil && np.IsErrExists(err1) {
		return false
	}
	// Try to claim the proc by removing it from the runq. If the remove is
	// successful, then we claimed the proc.
	if err := pd.Remove(procPath); err != nil {
		db.DPrintf("PROCD", "Failed to claim: %v", err)
		// If we didn't successfully claim the proc, but we *did* successfully
		// create the semaphore, then someone else must have created and then
		// removed the original one already. Remove/clean up the semaphore.
		if err1 == nil {
			semStart.Up()
		}
		return false
	}
	db.DPrintf("PROCD", "Sem init done: %v", p)
	return true
}
