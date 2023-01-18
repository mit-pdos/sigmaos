package schedd

import (
	db "sigmaos/debug"
	"sigmaos/proc"
)

func (sd *Schedd) stealProc(p *proc.Proc) bool {
	db.DFatalf("TODO")
	return false
}

//import (
//	sp "sigmaos/sigmap"
//)
//
//var WS_LC_QUEUE_PATH = path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_LC)
//var WS_BE_QUEUE_PATH = path.Join(sp.PROCD_WS, sp.PROCD_RUNQ_BE)
//
//// Thread in charge of stealing procs.
//func (pd *Procd) startWorkStealingMonitors() {
//	go pd.monitorWSQueue(sp.PROCD_RUNQ_LC)
//	go pd.monitorWSQueue(sp.PROCD_RUNQ_BE)
//}
//
//// Monitor a Work-Stealing queue.
//func (sd *Procd) monitorWSQueue(wsQueue string) {
//	wsQueuePath := path.Join(sp.PROCD_WS, wsQueue)
//	for !pd.readDone() {
//		// Wait for a bit to avoid overwhelming named.
//		time.Sleep(sp.Conf.Procd.WORK_STEAL_SCAN_TIMEOUT)
//		// Don't bother reading the BE queue if we couldn't possibly claim the
//		// proc.
//		if wsQueue == sp.PROCD_RUNQ_BE && !pd.canClaimBEProc() {
//			db.DPrintf(db.PROCD, "Skip monitoring BE WS queue because we can't claim another BE proc")
//			continue
//		}
//		var nremote int
//		stealable := make([]string, 0)
//		// Wait until there is a proc to steal.
//		sts, err := pd.ReadDirWatch(wsQueuePath, func(sts []*sp.Stat) bool {
//			// Discount procs already on this procd
//			for _, st := range sts {
//				// See if this proc was spawned on this procd or has been stolen. If
//				// so, discount it from the count of stealable procs.
//				b, err := pd.GetFile(path.Join(wsQueuePath, st.Name))
//				if err == nil {
//					if !strings.Contains(string(b), pd.memfssrv.MyAddr()) {
//						nremote++
//					}
//					stealable = append(stealable, st.Name)
//				}
//			}
//			db.DPrintf(db.PROCD, "Found %v stealable procs, of which %v belonged to other procds", len(stealable), nremote)
//			return len(stealable) == 0
//		})
//		// Version error may occur if another procd has modified the ws dir, and
//		// unreachable err may occur if the other procd is shutting down.
//		if err != nil && (serr.IsErrVersion(err) || serr.IsErrUnreachable(err)) {
//			db.DPrintf(db.PROCD_ERR, "Error ReadDirWatch: %v %v", err, len(sts))
//			db.DPrintf(db.ALWAYS, "Error ReadDirWatch: %v %v", err, len(sts))
//			continue
//		}
//		if err != nil {
//			pd.perf.Done()
//			db.DFatalf("Error ReadDirWatch: %v", err)
//		}
//		// Shuffle the queue of stealable procs.
//		rand.Shuffle(len(stealable), func(i, j int) {
//			stealable[i], stealable[j] = stealable[j], stealable[i]
//		})
//		// Store the queue of stealable procs for worker threads to read.
//		pd.Lock()
//		db.DPrintf(db.PROCD, "Waking %v worker procs to steal from %v", len(stealable), wsQueue)
//		pd.wsQueues[wsQueuePath] = stealable
//		// Wake up waiting workers to try to steal each proc.
//		for _ = range stealable {
//			pd.Signal()
//		}
//		pd.Unlock()
//	}
//}
//
//// Find if any procs spawned at this procd haven't been run in a while. If so,
//// offer them as stealable.
//func (pd *Procd) offerStealableProcs() {
//	// Store the procs this procd has already offered, and the runq they were
//	// stored in.
//	alreadyOffered := make(map[string]string)
//	for !pd.readDone() {
//		toOffer := make(map[string]string)
//		present := make(map[string]string)
//		// Wait for a bit.
//		time.Sleep(sp.Conf.Procd.STEALABLE_PROC_TIMEOUT)
//		runqs := []string{sp.PROCD_RUNQ_LC, sp.PROCD_RUNQ_BE}
//		for _, runq := range runqs {
//			runqPath := path.Join(sp.PROCD, pd.memfssrv.MyAddr(), runq)
//			_, err := pd.ProcessDir(runqPath, func(st *sp.Stat) (bool, error) {
//				// XXX Based on how we stuff Mtime into sp.Stat (at a second
//				// granularity), but this should be changed, perhaps.
//				// If proc has been hanging in the runq for too long, it is a candidate for work-stealing.
//				if uint32(time.Now().Unix())*1000 > st.Mtime*1000+uint32(sp.Conf.Procd.STEALABLE_PROC_TIMEOUT/time.Millisecond) {
//					// Don't re-offer procs which have already been offered.
//					if _, ok := alreadyOffered[st.Name]; !ok {
//						toOffer[st.Name] = runq
//					}
//					present[st.Name] = runq
//					alreadyOffered[st.Name] = runq
//				}
//				return false, nil
//			})
//			if err != nil {
//				pd.perf.Done()
//				db.DFatalf("Error ProcessDir: p %v err %v myIP %v", runqPath, err, pd.memfssrv.MyAddr())
//			}
//		}
//		//		db.DPrintf(db.PROCD, "Procd %v already offered %v", pd.memfssrv.MyAddr(), alreadyOffered)
//		for pid, runq := range toOffer {
//			db.DPrintf(db.PROCD, "Procd %v offering stealable proc %v", pd.memfssrv.MyAddr(), pid)
//			runqPath := path.Join(sp.PROCD, pd.memfssrv.MyAddr(), runq)
//			target := []byte(path.Join(runqPath, pid) + "/")
//			//			target := fslib.MakeTargetTree(pd.memfssrv.MyAddr(), []string{runq, pid})
//			link := path.Join(sp.PROCD_WS, runq, pid)
//			if err := pd.Symlink(target, link, 0777|sp.DMTMP); err != nil {
//				if serr.IsErrExists(err) {
//					db.DPrintf(db.PROCD, "Re-advertise symlink %v", target)
//				} else {
//					pd.perf.Done()
//					db.DFatalf("Error Symlink: %v", err)
//				}
//			}
//		}
//		// Clean up procs which are no longer in the queue.
//		for pid := range alreadyOffered {
//			// Proc is no longer in the queue, so forget about it.
//			if _, ok := present[pid]; !ok {
//				delete(alreadyOffered, pid)
//			}
//		}
//	}
//}
