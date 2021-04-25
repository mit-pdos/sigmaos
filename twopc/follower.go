package twopc

import (
	"log"
	"os"
	"sync"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type Follower struct {
	mu sync.Mutex
	*fslib.FsLib
	me    string
	twopc *Twopc
	txn   TxnI
}

func prepareName(flw string) string {
	return TWOPCPREPARED + flw
}

func commitName(flw string) string {
	return TWOPCCOMMITTED + flw
}

func MakeFollower(fsl *fslib.FsLib, me string, txn TxnI) (*Follower, error) {
	flw := &Follower{}
	log.Printf("MakeFollower %v\n", me)
	flw.me = me
	flw.FsLib = fsl
	flw.txn = txn

	if err := flw.MakeFile(DIR2PC+"/"+flw.me, 0777|np.DMTMP, nil); err != nil {
		log.Fatalf("MakeFile %v failed %v\n", COORD, err)
	}

	// set watch for twopcprep, indicating a transaction
	_, err := flw.readTwopcWatch(TWOPCPREP, flw.watchTwopcPrep)
	if err != nil {
		db.DLPrintf("FLW", "MakeFollower set watch on %v (err %v)\n", TWOPCPREP, err)
	}

	return flw, nil
}

func (flw *Follower) watchTwopcPrep(p string, err error) {
	db.DLPrintf("FLW", "Watch fires %v %v; prepare?\n", p, err)
	if err == nil {
		flw.prepare()
	} else {
		_, err = flw.readTwopcWatch(TWOPCPREP, flw.watchTwopcPrep)
		if err == nil {
			db.DLPrintf("FLW", "watchTwopcPrep: next trans %v (err %v)\n", TWOPCPREP, err)
			flw.prepare()
		} else {
			db.DLPrintf("FLW", "Commit: set watch on %v (err %v)\n", TWOPCPREP, err)
		}
	}
}

func (flw *Follower) readTwopcWatch(conffile string, f fsclnt.Watch) (*Twopc, error) {
	twopc := Twopc{}
	err := flw.ReadFileJsonWatch(conffile, &twopc, f)
	return &twopc, err
}

// Tell coord we are prepared to commit new config
func (flw *Follower) prepared(status string) {
	fn := prepareName(flw.me)
	db.DLPrintf("FLW", "Prepared %v\n", fn)
	err := flw.MakeFileAtomic(fn, 0777, []byte(status))
	if err != nil {
		db.DLPrintf("FLW", "Prepared: make file %v failed %v\n", fn, err)
	}
}

func (flw *Follower) committed() {
	fn := commitName(flw.me)
	db.DLPrintf("FLW", "Committed %v\n", fn)
	err := flw.MakeFile(fn, 0777, []byte("OK"))
	if err != nil {
		db.DLPrintf("FLW", "Committed: make file %v failed %v\n", fn, err)
	}
}

func (flw *Follower) watchTwopcCommit(p string, err error) {
	db.DLPrintf("FLW", "Watch conf fires %v %v; commit\n", p, err)
	flw.commit()
}

// XXX maybe check if one is already running?
func (flw *Follower) restartCoord() {
	log.Printf("FLW %v watchCoord: COORD crashed %v\n", flw.me, flw.twopc)
	flw.twopc = clean(flw.FsLib)
	if flw.twopc == nil {
		log.Printf("clean")
		return
	}
	pid1 := SpawnCoord(flw.FsLib, "restart", flw.twopc.Followers)
	ok, err := flw.Wait(pid1)
	if err != nil {
		log.Printf("FLW wait failed\n")
	}
	log.Printf("FLW Coord %v done %v\n", pid1, string(ok))

}

func (flw *Follower) watchCoord(p string, err error) {
	flw.mu.Lock()
	done := flw.twopc == nil
	flw.mu.Unlock()

	log.Printf("FLW Watch coord fires %v %v done? %v\n", p, err, done)

	// coord may have exited because it is done. if I am not in
	// 2PC, then assume coord exited because it is done.
	// clerks are able to do puts/gets.
	if done {
		return
	}

	if err == nil {
		flw.restartCoord()
	} else {
		// set remove watch on coord in case it crashes during 2PC
		err = flw.SetRemoveWatch(COORD, flw.watchCoord)
		if err != nil {
			flw.restartCoord()
		}
	}
}

func (flw *Follower) prepare() {
	flw.mu.Lock()

	var err error

	log.Printf("FLW %v prepare\n", flw.me)

	// set remove watch on coord in case it crashes during 2PC
	err = flw.SetRemoveWatch(COORD, flw.watchCoord)
	if err != nil {
		db.DLPrintf("FLW", "Prepare: COORD crashed\n")
	}

	_, err = flw.readTwopcWatch(TWOPCCOMMIT, flw.watchTwopcCommit)
	if err == nil {
		log.Fatalf("FLW %v: readTwopcWatch %v err %v\n", flw.me, TWOPCCOMMIT, err)
	}
	db.DLPrintf("FLW", "prepare: watch for %v\n", TWOPCCOMMIT)

	flw.twopc = readTwopc(flw.FsLib, TWOPCPREP)
	if flw.twopc == nil {
		log.Fatalf("FLW %v: FLW cannot read %v err %v\n", flw.me, TWOPCPREP, err)
	}

	db.DLPrintf("FLW", "prepare for new config: %v\n", flw.twopc)

	flw.mu.Unlock()

	err = flw.txn.Prepare()
	if err != nil {
		log.Printf("Prepare failed %v\n", err)
		os.Exit(1)
	}

	flw.prepared("OK")
}

func (flw *Follower) commit() {
	flw.mu.Lock()
	defer flw.mu.Unlock()

	log.Printf("FLW %v commit/abort\n", flw.me)

	flw.twopc = readTwopc(flw.FsLib, TWOPCCOMMIT)
	if flw.twopc == nil {
		log.Fatalf("FLW commit cannot read %v\n", TWOPCCOMMIT)
	}

	if flw.twopc.Status == TCOMMIT {
		log.Printf("%v: FLW commit twopc %v\n", flw.me, flw.twopc)
		flw.txn.Commit()
	} else {
		log.Printf("%v: FLW abort twopc %v\n", flw.me, flw.twopc)
		flw.txn.Abort()
	}

	flw.committed()
	flw.txn.Done()
}
