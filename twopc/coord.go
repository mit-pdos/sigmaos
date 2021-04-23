package twopc

//
// Coordinator for two-phase commit.  This is a short-lived daemon: it
// performs the transaction and then exits.
//

import (
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

const (
	TXNDIR       = "name/txn"
	COORD        = TXNDIR + "/coord"
	TXN          = TXNDIR + "/txn"
	TXNPREP      = TXNDIR + "/txnprep"
	TXNCOMMIT    = TXNDIR + "/txncommit"
	TXNPREPARED  = TXNDIR + "/prepared/"
	TXNCOMMITTED = TXNDIR + "/committed/"
	TXNLOCK      = "lock"
)

type Coord struct {
	*fslib.FsLib
	pid     string
	opcode  string
	txnprog string
	args    []string
	ch      chan Tstatus
	txn     *Trans
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("MakeCoord: too few arguments %v\n", args)
	}

	db.Name("coord")

	cd := &Coord{}
	cd.pid = args[0]
	cd.opcode = args[1]
	cd.txnprog = args[2]
	cd.args = args[3:]
	cd.ch = make(chan Tstatus)
	cd.FsLib = fslib.MakeFsLib("coord")

	// Grab TXNLOCK before starting coord
	if err := cd.LockFile(TXNDIR, TXNLOCK); err != nil {
		log.Fatalf("Lock failed %v\n", err)
	}

	log.Printf("lock\n")

	db.DLPrintf("COORD", "New coord %v %v", cd.txnprog, args)

	if err := cd.MakeFile(COORD, 0777|np.DMTMP, nil); err != nil {
		log.Fatalf("MakeFile %v failed %v\n", COORD, err)
	}

	cd.Started(cd.pid)
	return cd, nil
}

func (cd *Coord) exit() {
	log.Printf("unlock\n")

	if err := cd.Remove(COORD); err != nil {
		log.Printf("Remove %v failed %v\n", COORD, err)
	}

	if err := cd.UnlockFile(TXNDIR, TXNLOCK); err != nil {
		log.Fatalf("Unlock failed failed %v\n", err)
	}
}

func (cd *Coord) restart() {
	cd.txn = clean(cd.FsLib)
	if cd.txn == nil {
		log.Printf("clean\n")
		return
	}
	prepared := mkFlwsMapStatus(cd.FsLib, TXNPREPARED)
	committed := mkFlwsMapStatus(cd.FsLib, TXNCOMMITTED)

	db.DLPrintf("COORD", "Restart: txn %v prepared %v commit %v\n",
		cd.txn, prepared, committed)

	fws := mkFlwsMap(cd.FsLib, cd.txn.Followers)
	if fws.doCommit(prepared) {
		db.DLPrintf("COORD", "Restart: finish commit %d\n", committed.len())
		cd.commit(fws, committed.len(), true)
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
	nextFws.setStatusWatches(TXNPREPARED, cd.watchStatus)

	err := cd.MakeFileJsonAtomic(TXNPREP, 0777, *cd.txn)
	if err != nil {
		db.DLPrintf("COORD", "COORD: MakeFileJsonAtomic %v err %v\n",
			TXNCOMMIT, err)
	}

	// depending how many KVs ack, crash2 results
	// in a abort or commit
	if cd.opcode == "crash2" {
		db.DLPrintf("COORD", "Crash2\n")
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
		cd.txn.Status = TCOMMIT
		db.DLPrintf("COORD", "Commit to %v\n", cd.txn)
	} else {
		cd.txn.Status = TABORT
		db.DLPrintf("COORD", "Abort to %v\n", cd.txn)
	}

	if err := cd.WriteFileJson(TXNPREP, *cd.txn); err != nil {
		db.DLPrintf("COORD", "Write %v err %v\n", TXNPREP, err)
		return
	}

	fws.setStatusWatches(TXNCOMMITTED, cd.watchStatus)

	// commit/abort to new TXN, which maybe the same as the
	// old one
	err := cd.Rename(TXNPREP, TXNCOMMIT)
	if err != nil {
		db.DLPrintf("COORD", "COORD: rename %v -> %v: error %v\n",
			TXNPREP, TXNCOMMIT, err)
		return
	}

	// crash3 should results in commit (assuming no KVs crash)
	if cd.opcode == "crash3" {
		db.DLPrintf("COORD", "Crash3\n")
		os.Exit(1)
	}

	for i := 0; i < fws.len()-ndone; i++ {
		s := <-cd.ch
		db.DLPrintf("COORD", "KV commit status %v\n", s)
	}

	db.DLPrintf("COORD", "Done commit/abort\n")
}

func (cd *Coord) TwoPC() {
	defer cd.exit()

	log.Printf("COORD Coord: %v %v\n", cd.txnprog, cd.args)

	// db.DLPrintf("COORD", "Coord: %v\n", cd.args)

	// XXX set removeWatch on KVs? maybe in KV

	cd.restart()

	switch cd.opcode {
	case "restart":
		return
	}

	cd.txn = makeTrans(1, cd.args, cd.txnprog)

	fws := mkFlwsMap(cd.FsLib, cd.args)

	db.DLPrintf("COORD", "Coord txn %v %v\n", cd.txn, fws)

	if cd.args[0] == "crash1" {
		db.DLPrintf("COORD", "Crash1\n")
		os.Exit(1)
	}

	cd.Remove(TXNCOMMIT) // don't care if succeeds or not
	cd.rmStatusFiles(TXNPREPARED)
	cd.rmStatusFiles(TXNCOMMITTED)

	fws.setFlwsWatches(cd.watchFlw)

	log.Printf("COORD prepare\n")

	ok, n := cd.prepare(fws)

	log.Printf("COORD commit %v\n", ok)

	cd.commit(fws, fws.len()-n, ok)

	cd.Remove(TXNCOMMIT) // don't care if succeeds or not
}
