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
	//	"sigmaos/serr"
)

func (sd *Schedd) getScheddClnt(scheddIp string) *protdevclnt.ProtDevClnt {
	var pdc *protdevclnt.ProtDevClnt
	var ok bool
	if pdc, ok = sd.schedds[scheddIp]; !ok {
		var err error
		pdc, err = protdevclnt.MkProtDevClnt(sd.mfs.FsLib(), path.Join(sp.SCHEDD, scheddIp))
		if err != nil {
			db.DFatalf("Error make procd clnt: %v", err)
		}
		sd.schedds[scheddIp] = pdc
	}
	return pdc
}

// Try to steal a proc from another schedd. Returns true if successful.
func (sd *Schedd) tryStealProc(realm string, p *proc.Proc) bool {
	var q string
	switch p.GetType() {
	case proc.T_LC:
		q = sp.WS_RUNQ_LC
	case proc.T_BE:
		q = sp.WS_RUNQ_BE
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
	// Remove the proc from the ws queue.
	sd.mfs.FsLib().Remove(path.Join(q, p.GetPid().String()))
	// Create a file for the parent proc to wait on
	sd.postProcInQueue(p)
	// Steal from the original schedd.
	sreq := &proto.StealProcRequest{
		ScheddIp: sd.mfs.MyAddr(),
		Realm:    realm,
		PidStr:   p.GetPid().String(),
	}
	sres := &proto.StealProcResponse{}
	err := sd.getScheddClnt(p.ScheddIp).RPC("Procd.StealProc", sreq, sres)
	if err != nil {
		db.DFatalf("Error StealProc schedd: %v", err)
	}
	// If unsuccessful, remove from queue.
	if !sres.OK {
		sd.removeProcFromQueue(p)
		db.DPrintf(db.SCHEDD, "Failed to steal proc %v", p.GetPid())
		db.DPrintf(db.ALWAYS, "Failed to steal proc %v", p.GetPid())
	} else {
		db.DPrintf(db.SCHEDD, "Stole proc %v", p.GetPid())
		db.DPrintf(db.ALWAYS, "Stole proc %v", p.GetPid())
	}
	return sres.OK
}

// Monitor a Work-Stealing queue.
func (sd *Schedd) monitorWSQueue(wsQueue string, qtype proc.Ttype) {
	for {
		// Wait for a bit to avoid overwhelming named.
		time.Sleep(sp.Conf.Procd.WORK_STEAL_SCAN_TIMEOUT)
		stealable := make(map[string][]*proc.Proc, 0)
		// Wait until there is a proc to steal.
		sts, err := sd.mfs.FsLib().ReadDirWatch(wsQueue, func(sts []*sp.Stat) bool {
			sd.mu.Lock()
			defer sd.mu.Unlock()

			var nStealable int
			for _, st := range sts {
				// Read and unmarshal proc.
				b, err := sd.mfs.FsLib().GetFile(path.Join(wsQueue, st.Name))
				if err != nil {
					// Proc may have been stolen already.
					continue
				}
				p := proc.MakeEmptyProc()
				p.Unmarshal(b)

				// Is the proc a local proc? If so, don't add it to the queue of
				// stealable procs.
				if _, ok := sd.qs[p.Realm].pmap[proc.Tpid(st.Name)]; ok {
					continue
				}
				if _, ok := stealable[p.Realm]; !ok {
					stealable[p.Realm] = make([]*proc.Proc, 0)
				}
				// Add to the list of stealable procs
				stealable[p.Realm] = append(stealable[p.Realm], p)
				nStealable++
			}
			db.DPrintf(db.SCHEDD, "Found %v stealable procs %v", nStealable, stealable)
			return nStealable == 0
		})
		// TODO: special-case error handling?
		if err != nil { //&& (serr.IsErrVersion(err) || serr.IsErrUnreachable(err)) {
			db.DPrintf(db.SCHEDD_ERR, "Error ReadDirWatch: %v %v", err, len(sts))
			db.DFatalf("Error ReadDirWatch: %v %v", err, len(sts))
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
		db.DPrintf(db.SCHEDD, "Waking %v worker procs to steal from %v", len(stealable), wsQueue)
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
		// Wake up scheduler thread.
		// TODO: don't wake up if stealable procs aren't new?
		sd.cond.Signal()
		sd.mu.Unlock()
	}
}

// Find if any procs spawned at this schedd haven't been run in a while. If so,
// offer them as stealable.
func (sd *Schedd) offerStealableProcs() {
	// Store the procs this schedd has already offered to avoid re-offering them.
	// TODO: clear this list occasionally.
	alreadyOffered := make(map[proc.Tpid]bool)
	for {
		toOffer := make([]*proc.Proc, 0)
		// Wait for a bit.
		time.Sleep(sp.Conf.Procd.STEALABLE_PROC_TIMEOUT)
		sd.mu.Lock()
		for _, q := range sd.qs {
			// Iterate the procs in each realm's queue.
			for _, p := range q.pmap {
				// If this proc was already offered, skip it.
				if _, ok := alreadyOffered[p.GetPid()]; ok {
					continue
				}
				// If this proc has not been spawned for a long time, prepare to offer
				// it as stealable.
				if time.Since(p.GetSpawnTime()) >= sp.Conf.Procd.STEALABLE_PROC_TIMEOUT {
					toOffer = append(toOffer, p)
				}
			}
		}
		sd.mu.Unlock()
		for _, p := range toOffer {
			alreadyOffered[p.GetPid()] = true
			var q string
			switch p.GetType() {
			case proc.T_LC:
				q = sp.WS_RUNQ_LC
			case proc.T_BE:
				q = sp.WS_RUNQ_BE
			default:
				db.DFatalf("Unrecognized proc type: %v", p.GetType())
			}
			if _, err := sd.mfs.FsLib().PutFile(path.Join(q, p.GetPid().String()), 0777, sp.OWRITE, p.Marshal()); err != nil {
				db.DFatalf("Error PutFile: %v", err)
			}
		}
	}
}
