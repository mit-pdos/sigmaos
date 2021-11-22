package twopc

//
// Coordinator for two-phase commit.  This is a short-lived daemon: it
// performs the transaction and then exits.
//

import (
	"fmt"
	"log"
	"os"

	"ulambda/atomic"
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/sync"
)

const (
	DIR2PC         = "name/twopc"
	COORD          = DIR2PC + "/coord"
	TWOPC          = DIR2PC + "/twopc"
	TWOPCPREP      = DIR2PC + "/twopcprep"
	TWOPCCOMMIT    = DIR2PC + "/twopccommit"
	TWOPCPREPARED  = DIR2PC + "/prepared/"
	TWOPCCOMMITTED = DIR2PC + "/committed/"
	TWOPCLOCK      = "lock"
)

type Coord struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	opcode    string
	args      []string
	ch        chan Tstatus
	twopc     *Twopc
	twopclock *sync.Lock
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) < 2 {

		return nil, fmt.Errorf("MakeCoord: too few arguments %v\n", args)
	}

	db.Name("coord")

	cd := &Coord{}
	cd.opcode = args[0]
	cd.args = args[1:]
	cd.ch = make(chan Tstatus)
	cd.FsLib = fslib.MakeFsLib("coord")
	cd.ProcClnt = procclnt.MakeProcClnt(cd.FsLib)
	cd.twopclock = sync.MakeLock(cd.FsLib, DIR2PC, TWOPCLOCK, true)

	// Grab TWOPCLOCK before starting coord
	cd.twopclock.Lock()

	log.Printf("COORD lock %v\n", args)

	db.DLPrintf("COORD", "New coord %v", args)

	if err := cd.MakeFile(COORD, 0777|np.DMTMP, np.OWRITE, nil); err != nil {
		log.Fatalf("MakeFile %v failed %v\n", COORD, err)
	}

	cd.Started(proc.GetPid())
	return cd, nil
}

func (cd *Coord) exit() {
	log.Printf("unlock\n")

	if err := cd.Remove(COORD); err != nil {
		log.Printf("Remove %v failed %v\n", COORD, err)
	}

	cd.twopclock.Unlock()
}

func (cd *Coord) restart() {
	cd.twopc = clean(cd.FsLib)
	if cd.twopc == nil {
		log.Printf("COORD clean\n")
		return
	}
	prepared := mkFlwsMapStatus(cd.FsLib, TWOPCPREPARED)
	committed := mkFlwsMapStatus(cd.FsLib, TWOPCCOMMITTED)

	db.DLPrintf("COORD", "Restart: twopc %v prepared %v commit %v\n",
		cd.twopc, prepared, committed)

	fws := mkFlwsMap(cd.FsLib, cd.twopc.Participants)
	if fws.doCommit(prepared) {

		if committed.len() == fws.len() {
			db.DLPrintf("COORD", "Restart: finished commit %d\n", committed.len())
			cd.cleanup()
		} else {
			db.DLPrintf("COORD", "Restart: finish commit %d\n", committed.len())
			cd.commit(fws, committed.len(), true)
		}
	} else {
		db.DLPrintf("COORD", "Restart: abort\n")
		cd.commit(fws, committed.len(), false)
	}
}

func (cd *Coord) rmStatusFiles(dir string) {
	sts, err := cd.ReadDir(dir)
	if err != nil {
		log.Fatalf("COORD: ReadDir commit error %v\n", err)
	}
	for _, st := range sts {
		fn := dir + st.Name
		err = cd.Remove(fn)
		if err != nil {
			db.DLPrintf("COORD", "Remove %v failed %v\n", fn, err)
		}
	}
}

func (cd *Coord) watchStatus(p string, err error) {
	db.DLPrintf("COORD", "watchStatus %v\n", p)
	status := TABORT
	b, err := cd.ReadFile(p)
	if err != nil {
		db.DLPrintf("COORD", "watchStatus ReadFile %v err %v\n", p, b)
	}
	if string(b) == "OK" {
		status = TCOMMIT
	}
	cd.ch <- status
}

func (cd *Coord) watchFlw(p string, err error) {
	db.DLPrintf("COORD", "watchFlw %v\n", p)
	cd.ch <- TCRASH
}

func (cd *Coord) prepare(nextFws *FlwsMap) (bool, int) {
	nextFws.setStatusWatches(TWOPCPREPARED, cd.watchStatus)

	err := atomic.MakeFileJsonAtomic(cd.FsLib, TWOPCPREP, 0777, *cd.twopc)
	if err != nil {
		db.DLPrintf("COORD", "COORD: MakeFileJsonAtomic %v err %v\n",
			TWOPCCOMMIT, err)
	}

	// depending how many KVs ack, crash3 results
	// in a abort or commit
	if cd.opcode == "crash3" {
		db.DLPrintf("COORD", "Crash3\n")
		os.Exit(1)
	}

	success := true
	n := 0
	for i := 0; i < nextFws.len(); i++ {
		status := <-cd.ch
		switch status {
		case TCOMMIT:
			db.DLPrintf("COORD", "KV prepared\n")
			n += 1
		case TABORT:
			db.DLPrintf("COORD", "KV aborted\n")
			n += 1
			success = false
		default:
			db.DLPrintf("COORD", "KV crashed\n")
			success = false
		}
	}
	return success, n
}

func (cd *Coord) commit(fws *FlwsMap, ndone int, ok bool) {
	if ok {
		cd.twopc.Status = TCOMMIT
		db.DLPrintf("COORD", "Commit to %v\n", cd.twopc)
	} else {
		cd.twopc.Status = TABORT
		db.DLPrintf("COORD", "Abort to %v\n", cd.twopc)
	}

	if err := cd.WriteFileJson(TWOPCPREP, *cd.twopc); err != nil {
		db.DLPrintf("COORD", "Write %v err %v\n", TWOPCPREP, err)
		return
	}

	fws.setStatusWatches(TWOPCCOMMITTED, cd.watchStatus)

	// commit/abort to new TWOPC, which maybe the same as the
	// old one
	err := cd.Rename(TWOPCPREP, TWOPCCOMMIT)
	if err != nil {
		db.DLPrintf("COORD", "COORD: rename %v -> %v: error %v\n",
			TWOPCPREP, TWOPCCOMMIT, err)
		return
	}

	// crash4 should results in commit (assuming no KVs crash)
	if cd.opcode == "crash4" {
		db.DLPrintf("COORD", "Crash4\n")
		os.Exit(1)
	}

	for i := 0; i < fws.len()-ndone; i++ {
		s := <-cd.ch
		db.DLPrintf("COORD", "KV commit status %v\n", s)
	}

	db.DLPrintf("COORD", "Done commit/abort\n")

	cd.cleanup()
}

func (cd *Coord) TwoPC() {
	defer cd.exit()

	log.Printf("COORD Coord: %v\n", cd.args)

	db.DLPrintf("COORD", "Coord: %v\n", cd.args)

	// XXX set removeWatch on KVs? maybe in KV

	cd.restart()

	switch cd.opcode {
	case "restart":
		return
	}

	cd.twopc = makeTwopc(1, cd.args)

	fws := mkFlwsMap(cd.FsLib, cd.args)

	db.DLPrintf("COORD", "Coord twopc %v %v\n", cd.twopc, fws)

	if cd.opcode == "crash2" {
		log.Printf("crash2\n")
		db.DLPrintf("COORD", "Crash2\n")
		os.Exit(1)
	}

	cd.Remove(TWOPCCOMMIT) // don't care if succeeds or not
	cd.rmStatusFiles(TWOPCPREPARED)
	cd.rmStatusFiles(TWOPCCOMMITTED)

	fws.setFlwsWatches(cd.watchFlw)

	log.Printf("COORD prepare\n")

	ok, n := cd.prepare(fws)

	log.Printf("COORD commit %v\n", ok)

	cd.commit(fws, fws.len()-n, ok)
}

func (cd *Coord) cleanup() {
	log.Printf("COORD cleanup %v\n", TWOPCCOMMIT)
	cd.Remove(TWOPCCOMMIT) // don't care if succeeds or not
}

func (cd *Coord) Exit() {
	cd.Exited(proc.GetPid(), "OK")
}
