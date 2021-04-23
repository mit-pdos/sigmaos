package twopc

import (
	"fmt"
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
	done chan bool
	pid  string
	me   string
	args []string
	txn  *Trans
}

func prepareName(flw string) string {
	return TXNPREPARED + flw
}

func commitName(flw string) string {
	return TXNCOMMITTED + flw
}

func flwname(pid string) string {
	return "flw" + pid
}

func MakeFollower(args []string) (*Follower, error) {
	flw := &Follower{}
	flw.done = make(chan bool)
	if len(args) != 2 {
		return nil, fmt.Errorf("MakeFollower: too few arguments %v\n", args)
	}
	log.Printf("MakeFollower %v\n", args)
	flw.pid = args[0]
	flw.args = args
	flw.me = flwname(flw.pid)
	db.Name(flw.me)
	flw.FsLib = fslib.MakeFsLib("follower")

	if err := flw.MakeFile(TXNDIR+"/"+flw.me, 0777|np.DMTMP, nil); err != nil {
		log.Fatalf("MakeFile %v failed %v\n", COORD, err)
	}

	// set watch for txnprep, indicating a transaction
	_, err := flw.readTxnWatch(TXNPREP, flw.watchTxnPrep)
	if err != nil {
		db.DLPrintf("FLW", "MakeFollower set watch on %v (err %v)\n", TXNPREP, err)
	}

	flw.Started(flw.pid)

	return flw, nil
}

func (flw *Follower) watchTxnPrep(p string, err error) {
	db.DLPrintf("FLW", "Watch fires %v %v; prepare?\n", p, err)
	if err == nil {
		flw.prepare()
	} else {
		_, err = flw.readTxnWatch(TXNPREP, flw.watchTxnPrep)
		if err == nil {
			db.DLPrintf("FLW", "watchTxnPrep: next trans %v (err %v)\n", TXNPREP, err)
			flw.prepare()
		} else {
			db.DLPrintf("FLW", "Commit: set watch on %v (err %v)\n", TXNPREP, err)
		}
	}
}

func (flw *Follower) readTxnWatch(conffile string, f fsclnt.Watch) (*Trans, error) {
	txn := Trans{}
	err := flw.ReadFileJsonWatch(conffile, &txn, f)
	return &txn, err
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

func (flw *Follower) watchTxnCommit(p string, err error) {
	db.DLPrintf("FLW", "Watch conf fires %v %v; commit\n", p, err)
	flw.commit()
}

// XXX maybe check if one is already running?
func (flw *Follower) restartCoord() {
	log.Printf("FLW %v watchCoord: COORD crashed\n", flw.me)
	pid1 := spawnCoord(flw.FsLib, []string{"restart", flw.me})
	ok, err := flw.Wait(pid1)
	if err != nil {
		log.Printf("FLW wait failed\n")
	}
	log.Printf("FLW Coord done %v\n", string(ok))

}

func (flw *Follower) watchCoord(p string, err error) {
	flw.mu.Lock()
	done := flw.txn == nil
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

	_, err = flw.readTxnWatch(TXNCOMMIT, flw.watchTxnCommit)
	if err == nil {
		log.Fatalf("FLW %v: readTxnWatch %v err %v\n", flw.me, TXNCOMMIT, err)
	}
	db.DLPrintf("FLW", "prepare: watch for %v\n", TXNCOMMIT)

	flw.txn = readTrans(flw.FsLib, TXNPREP)
	if flw.txn == nil {
		log.Fatalf("FLW %v: FLW cannot read %v err %v\n", flw.me, TXNPREP, err)
	}

	db.DLPrintf("FLW", "prepare for new config: %v\n", flw.txn)

	flw.mu.Unlock()

	// XXX do trans

	if flw.args[1] == "crash4" {
		db.DLPrintf("FLW", "Crashed in prepare\n")
		os.Exit(1)
	}

	flw.prepared("OK")
}

func (flw *Follower) commit() {
	flw.mu.Lock()
	defer flw.mu.Unlock()

	log.Printf("FLW %v commit/abort\n", flw.me)

	txn := readTrans(flw.FsLib, TXNCOMMIT)
	if txn == nil {
		log.Fatalf("FLW commit cannot read %v\n", TXNCOMMIT)
	}

	if txn.Status == TCOMMIT {
		log.Printf("%v: FLW commit txn %v\n", flw.me, txn)
	} else {
		log.Printf("%v: FLW abort txn %v\n", flw.me, txn)
	}

	if flw.args[1] == "crash5" {
		db.DLPrintf("FLW", "Crashed in commit/abort\n")
		os.Exit(1)
	}

	flw.committed()
	flw.done <- true
}

func (flw *Follower) Work() {
	db.DLPrintf("FLW", "Work\n")
	<-flw.done
	db.DLPrintf("FLW", "exit\n")
}
