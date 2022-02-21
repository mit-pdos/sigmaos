package twopc

import (
	"log"
	"os"
	"sync"

	"ulambda/atomic"
	db "ulambda/debug"
	"ulambda/fenceclnt"
	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/procclnt"
)

type Participant struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	fclnt  *fenceclnt.FenceClnt
	me     string
	twopc  *Twopc
	txn    TxnI
	opcode string
}

func prepareName(flw string) string {
	return TWOPCPREPARED + flw
}

func commitName(flw string) string {
	return TWOPCCOMMITTED + flw
}

func MakeParticipant(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, me string, txn TxnI, opcode string) (*Participant, error) {
	p := &Participant{}
	log.Printf("PART MakeParticipant %v %v\n", me, opcode)
	p.me = me
	p.FsLib = fsl
	p.ProcClnt = pclnt
	p.fclnt = fenceclnt.MakeFenceClnt(p.FsLib, TWOPCFENCE, 0, []string{DIR2PC})
	p.txn = txn
	p.opcode = opcode

	if _, err := p.PutFile(DIR2PC+"/"+p.me, 0777|np.DMTMP, np.OWRITE, nil); err != nil {
		log.Fatalf("MakeFile %v failed %v\n", COORD, err)
	}

	// set watch for twopcprep, indicating a transaction
	if _, err := p.readTwopcWatch(TWOPCPREP, p.watchTwopcPrep); err != nil {
		db.DLPrintf("PART", "MakeParticipant set watch on %v (err %v)\n", TWOPCPREP, err)
	}

	return p, nil
}

func (p *Participant) watchTwopcPrep(path string, err error) {
	db.DLPrintf("PART", "Watch fires %v %v; prepare?\n", path, err)
	if err == nil {
		p.prepare()
	} else {
		_, err = p.readTwopcWatch(TWOPCPREP, p.watchTwopcPrep)
		if err == nil {
			db.DLPrintf("PART", "watchTwopcPrep: next trans %v (err %v)\n", TWOPCPREP, err)
			p.prepare()
		} else {
			db.DLPrintf("PART", "Commit: set watch on %v (err %v)\n", TWOPCPREP, err)
		}
	}
}

func (p *Participant) readTwopcWatch(conffile string, f fsclnt.Watch) (*Twopc, error) {
	twopc := Twopc{}
	err := p.GetFileJsonWatch(conffile, &twopc)
	return &twopc, err
}

// Tell coord we are prepared to commit new config
func (p *Participant) prepared(status string) {
	fn := prepareName(p.me)
	db.DLPrintf("PART", "Prepared %v\n", fn)
	err := atomic.PutFileAtomic(p.FsLib, fn, 0777, []byte(status))
	if err != nil {
		db.DLPrintf("PART", "Prepared: make file %v failed %v\n", fn, err)
	}
}

func (p *Participant) committed() {
	fn := commitName(p.me)
	db.DLPrintf("PART", "Committed %v\n", fn)
	_, err := p.PutFile(fn, 0777, np.OWRITE, []byte("OK"))
	if err != nil {
		db.DLPrintf("PART", "Committed: make file %v failed %v\n", fn, err)
	}
}

func (p *Participant) watchTwopcCommit(path string, err error) {
	db.DLPrintf("PART", "Watch conf fires %v %v; commit\n", path, err)
	p.commit()
}

func (p *Participant) restartCoord() {
	log.Printf("PART %v restartCoord: COORD crashed %v\n", p.me, p.twopc)

	if err := p.fclnt.ReleaseFence(); err != nil {
		log.Fatalf("Error ReleaseFence restartCoord: %v", err)
	}
	// Grab fence again
	if b, err := p.fclnt.AcquireFenceR(); err != nil {
		log.Fatalf("Error AcquireFenceR  restartCoord: %v, %v", b, err)
	}

	p.twopc = clean(p.FsLib)

	// set watch for twopcprep, indicating a transaction
	if _, err := p.readTwopcWatch(TWOPCPREP, p.watchTwopcPrep); err != nil {
		db.DLPrintf("PART", "MakeParticipant set watch on %v (err %v)\n", TWOPCPREP, err)
	}
}

func (p *Participant) watchCoord(path string, err error) {
	p.mu.Lock()
	done := p.twopc == nil
	p.mu.Unlock()

	log.Printf("PART Watch coord fires %v %v done? %v\n", path, err, done)

	// coord may have exited because it is done. if I am not in
	// 2PC, then assume coord exited because it is done.
	// clerks are able to do puts/gets.
	if done {
		return
	}

	if err == nil {
		p.restartCoord()
	} else {
		// set remove watch on coord in case it crashes during 2PC
		err = p.SetRemoveWatch(COORD, p.watchCoord)
		if err != nil {
			p.restartCoord()
		}
	}
}

func (p *Participant) prepare() {
	p.mu.Lock()
	// Grab fence before preparing
	if b, err := p.fclnt.AcquireFenceR(); err != nil {
		log.Fatalf("Error AcquireFenceR wait: %v, %v", b, err)
	}

	var err error

	log.Printf("PART %v prepare\n", p.me)

	// set remove watch on coord in case it crashes during 2PC
	err = p.SetRemoveWatch(COORD, p.watchCoord)
	if err != nil {
		db.DLPrintf("PART", "Prepare: COORD crashed\n")
		p.restartCoord()
		// return
	}

	_, err = p.readTwopcWatch(TWOPCCOMMIT, p.watchTwopcCommit)
	if err == nil {
		log.Fatalf("PART %v: readTwopcWatch %v err %v\n", p.me, TWOPCCOMMIT, err)
	}
	db.DLPrintf("PART", "prepare: watch for %v\n", TWOPCCOMMIT)

	p.twopc = readTwopc(p.FsLib, TWOPCPREP)
	if p.twopc == nil {
		log.Fatalf("PART %v: PART cannot read %v err %v\n", p.me, TWOPCPREP, err)
	}

	db.DLPrintf("PART", "prepare for new config: %v\n", p.twopc)

	p.mu.Unlock()

	err = p.txn.Prepare()
	if err != nil {
		log.Printf("Prepare failed %v\n", err)
		os.Exit(1)
	}

	if p.opcode == "crash1" {
		db.DLPrintf("PART", "Crashed in prepare\n")
		os.Exit(1)
	}

	p.prepared("OK")
}

func (p *Participant) commit() {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("PART %v commit/abort\n", p.me)

	p.twopc = readTwopc(p.FsLib, TWOPCCOMMIT)
	if p.twopc == nil {
		log.Fatalf("PART commit cannot read %v\n", TWOPCCOMMIT)
	}

	if p.twopc.Status == TCOMMIT {
		log.Printf("%v: PART commit twopc %v\n", p.me, p.twopc)
		p.txn.Commit()
	} else {
		log.Printf("%v: PART abort twopc %v\n", p.me, p.twopc)
		p.txn.Abort()
	}

	p.committed()

	// Release fence
	err := p.fclnt.ReleaseFence()
	if err != nil {
		log.Fatalf("Error Rlease release: %v", err)
	}

	p.txn.Done()
}
