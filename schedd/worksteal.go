package schedd

import (
	"math/rand"
	"path"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	proto "sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
)

func (sd *Schedd) getScheddClnt(scheddIp string) *protdevclnt.ProtDevClnt {
	var pdc *protdevclnt.ProtDevClnt
	var ok bool
	if pdc, ok = sd.schedds[scheddIp]; !ok {
		var err error
		pdc, err = protdevclnt.MkProtDevClnt(sd.mfs.SigmaClnt().FsLib, path.Join(sp.SCHEDD, scheddIp))
		if err != nil {
			db.DFatalf("Error make procd clnt: %v", err)
		}
		sd.schedds[scheddIp] = pdc
	}
	return pdc
}

// Try to steal a proc from another schedd. Returns true if successful.
func (sd *Schedd) tryStealProc(realm sp.Trealm, p *proc.Proc) bool {
	// Try to steal from the victim schedd.
	sreq := &proto.StealProcRequest{
		ScheddIp: sd.mfs.MyAddr(),
		Realm:    realm.String(),
		PidStr:   p.GetPid().String(),
	}
	sres := &proto.StealProcResponse{}
	err := sd.getScheddClnt(p.ScheddIp).RPC("Procd.StealProc", sreq, sres)
	if err != nil {
		db.DFatalf("Error StealProc schedd: %v", err)
	}
	if sres.OK {
		db.DPrintf(db.SCHEDD, "Stole proc %v", p.GetPid())
	} else {
		db.DPrintf(db.SCHEDD, "Failed to steal proc %v", p.GetPid())
	}
	sd.pmgr.TryStealProc(p)
	return sres.OK
}

// Monitor a Work-Stealing queue.
func (sd *Schedd) monitorWSQueue(qtype proc.Ttype) {
	for {
		// Wait for a bit to avoid overwhelming named.
		time.Sleep(sp.Conf.Procd.WORK_STEAL_SCAN_TIMEOUT)
		var stealable map[sp.Trealm][]*proc.Proc
		var ok bool
		// If there was a version error triggered while reading the queue, reread
		// it.
		if stealable, ok = sd.pmgr.GetWSQueue(qtype); !ok {
			continue
		}
		// Shuffle the queues of stealable procs.
		for _, q := range stealable {
			rand.Shuffle(len(q), func(i, j int) {
				q[i], q[j] = q[j], q[i]
			})
		}
		// Store the queue of stealable procs for worker threads to read.
		sd.mu.Lock()
		db.DPrintf(db.SCHEDD, "Waking %v worker procs to steal from %v", len(stealable), qtype)
		for r, q := range stealable {
			if _, ok := sd.qs[r]; !ok {
				sd.qs[r] = makeQueue()
			}
			switch qtype {
			case proc.T_LC:
				sd.qs[r].lcws = q
			case proc.T_BE:
				sd.qs[r].bews = q
			default:
				db.DFatalf("Unrecognized queue type: %v", qtype)
			}
		}
		// TODO: don't wake up if stealable procs aren't new?
		// Wake up scheduler thread.
		sd.cond.Signal()
		sd.mu.Unlock()
	}
}

// Find if any procs spawned at this schedd haven't been run in a while. If so,
// offer them as stealable.
func (sd *Schedd) offerStealableProcs() {
	// Store the procs this schedd has already offered to avoid re-offering them.
	alreadyOffered := make(map[proc.Tpid]bool)
	for {
		toOffer := make(map[proc.Tpid]*proc.Proc)
		// Wait for a bit.
		time.Sleep(sp.Conf.Procd.STEALABLE_PROC_TIMEOUT)
		sd.mu.Lock()
		for _, q := range sd.qs {
			// Iterate the procs in each realm's queue.
			for _, p := range q.pmap {
				// If this proc has not been spawned for a long time, prepare to offer
				// it as stealable.
				if time.Since(p.GetSpawnTime()) >= sp.Conf.Procd.STEALABLE_PROC_TIMEOUT {
					toOffer[p.GetPid()] = p
				}
			}
		}
		sd.mu.Unlock()
		for pid, _ := range alreadyOffered {
			// If this proc is no longer in the queue (it is not offerable), then
			// remove it from the alreadyOffered, since it will never be offered
			// again.
			if _, ok := toOffer[pid]; !ok {
				delete(alreadyOffered, pid)
			}
		}
		for _, p := range toOffer {
			// If this proc has not been offered already, then offer it.
			if _, ok := alreadyOffered[p.GetPid()]; !ok {
				alreadyOffered[p.GetPid()] = true
				sd.pmgr.OfferStealableProc(p)
			}
		}
	}
}
