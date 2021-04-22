package kv

//
// Shard coordinator: assigns shards to KVs using two-phase commit.
// This is a short-lived daemon: it rebalances shards and then exists.
//

import (
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/memfsd"
)

const (
	NSHARD       = 10
	KVDIR        = "name/kv"
	COORD        = KVDIR + "/coord"
	KVCONFIG     = KVDIR + "/config"
	KVCONFIGTMP  = KVDIR + "/configtmp"
	KVNEXTCONFIG = KVDIR + "/nextconfig"
	KVPREPARED   = KVDIR + "/prepared/"
	KVCOMMITTED  = KVDIR + "/committed/"
	KVLOCK       = "lock"
)

type Coord struct {
	*fslib.FsLibSrv
	pid      string
	args     []string
	ch       chan Tstatus
	conf     *Config
	nextConf *Config
}

func MakeCoord(args []string) (*Coord, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("MakeCoord: too few arguments %v\n", args)
	}
	cd := &Coord{}
	cd.pid = args[0]
	cd.args = args[1:]
	cd.ch = make(chan Tstatus)

	db.Name("coord")

	// Grab KVLOCK before starting coord
	fsl := fslib.MakeFsLib(COORD)
	if err := fsl.LockFile(KVDIR, KVLOCK); err != nil {
		log.Fatalf("Lock failed %v\n", err)
	}

	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("MakeCoord: no IP %v\n", err)
	}
	fsd := memfsd.MakeFsd(ip + ":0")
	db.DLPrintf("COORD", "New coord %v", args)
	fls, err := fslib.InitFsFsl(COORD, fsl, fsd, nil)
	if err != nil {
		return nil, err
	}
	cd.FsLibSrv = fls
	cd.Started(cd.pid)
	return cd, nil
}

func (cd *Coord) Exit() {
	cd.ExitFs(COORD)
}

func (cd *Coord) unlock() {
	if err := cd.UnlockFile(KVDIR, KVLOCK); err != nil {
		log.Fatalf("Unlock failed failed %v\n", err)
	}
}

func (cd *Coord) restart() {
	cd.conf = readConfig(cd.FsLibSrv, KVCONFIG)

	if cd.nextConf == nil {
		// either commit/aborted or never started
		db.DLPrintf("COORD", "Restart: clean\n")
		return
	}

	cd.nextConf = readConfig(cd.FsLibSrv, KVNEXTCONFIG)
	prepared := mkFollowersStatus(cd.FsLibSrv.Clnt(), KVPREPARED)
	committed := mkFollowersStatus(cd.FsLibSrv.Clnt(), KVCOMMITTED)

	db.DLPrintf("COORD", "Restart: conf %v next %v prepared %v commit %v\n",
		cd.conf, cd.nextConf, prepared, committed)

	fws := mkFollowers(cd.FsLibSrv.Clnt(), cd.nextConf.Shards)
	if fws.doCommit(prepared) {
		db.DLPrintf("COORD", "Restart: finish commit %d\n", committed.len())
		cd.commit(fws, committed.len(), true)
	} else {
		db.DLPrintf("COORD", "Restart: abort\n")
		fws := mkFollowers(cd.FsLibSrv.Clnt(), cd.conf.Shards)
		cd.commit(fws, committed.len(), false)
	}
}

func (cd *Coord) initShards(exclKvs []string) bool {
	db.DLPrintf("COORD", "initShards %v\n", exclKvs)
	excl := make(map[string]bool)
	for _, kv := range exclKvs {
		excl[kv] = true
	}
	for s, kv := range cd.conf.Shards {
		if _, ok := excl[kv]; ok { // shard s has been lost
			kvd := cd.nextConf.Shards[s]
			dst := shardPath(kvd, s, cd.nextConf.N)
			db.DLPrintf("COORD: Init shard dir %v\n", dst)
			err := cd.Mkdir(dst, 0777)
			if err != nil {
				db.DLPrintf("KV", "initShards: mkdir %v err %v\n", dst, err)
				return false
			}
		}
	}
	return true
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
	status := ABORT
	b, err := cd.ReadFile(p)
	if err != nil {
		db.DLPrintf("COORD", "watchStatus ReadFile %v err %v\n", p, b)
	}
	if string(b) == "OK" {
		status = COMMIT
	}
	cd.ch <- status
}

func (cd *Coord) watchKV(p string, err error) {
	db.DLPrintf("COORD", "watchKV %v\n", p)
	cd.ch <- CRASH
}

func (cd *Coord) prepare(nextFws *Followers) (bool, int) {
	nextFws.setStatusWatches(KVPREPARED, cd.watchStatus)

	err := cd.MakeFileJsonAtomic(KVNEXTCONFIG, 0777, *cd.nextConf)
	if err != nil {
		db.DLPrintf("COORD", "COORD: MakeFileJsonAtomic %v err %v\n",
			KVNEXTCONFIG, err)
	}

	// depending how many KVs ack, crash2 results
	// in a abort or commit
	if cd.args[0] == "crash2" {
		db.DLPrintf("COORD", "Crash2\n")
		os.Exit(1)
	}

	success := true
	n := 0
	for i := 0; i < nextFws.len(); i++ {
		status := <-cd.ch
		switch status {
		case COMMIT:
			db.DLPrintf("COORD", "KV prepared\n")
			n += 1
		case ABORT:
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

func (cd *Coord) commit(fws *Followers, ndone int, ok bool) {
	if ok {
		db.DLPrintf("COORD", "Commit to %v\n", cd.nextConf)
	} else {
		// Rename KVCONFIGTMP into KVNEXTCONFIG so that the followers
		// will abort to the old KVCONFIG
		if err := cd.CopyFile(KVCONFIG, KVCONFIGTMP); err != nil {
			db.DLPrintf("COORD", "CopyFile failed %v\n", err)
		}
		err := cd.Rename(KVCONFIGTMP, KVNEXTCONFIG)
		if err != nil {
			db.DLPrintf("COORD", "COORD: rename %v -> %v: error %v\n",
				KVCONFIGTMP, KVNEXTCONFIG, err)
			return
		}
		db.DLPrintf("COORD", "Abort to %v\n", cd.conf)
	}

	fws.setStatusWatches(KVCOMMITTED, cd.watchStatus)

	// commit/abort to new KVCONFIG, which maybe the same as the
	// old one
	err := cd.Rename(KVNEXTCONFIG, KVCONFIG)
	if err != nil {
		db.DLPrintf("COORD", "COORD: rename %v -> %v: error %v\n",
			KVNEXTCONFIG, KVCONFIG, err)
		return
	}

	// crash3 should results in commit (assuming no KVs crash)
	if cd.args[0] == "crash3" {
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
	defer cd.unlock() // release lock acquired in MakeCoord()

	// db.DLPrintf("COORD", "Coord: %v\n", cd.args)
	log.Printf("COORD Coord: %v\n", cd.args)

	// XXX set removeWatch on KVs? maybe in KV

	cd.restart()

	// We may have committed/aborted; reread cd.conf to get new
	// this config
	cd.conf = readConfig(cd.FsLibSrv, KVCONFIG)

	nextFws := mkFollowers(cd.FsLibSrv.Clnt(), cd.conf.Shards)

	switch cd.args[0] {
	case "crash1", "crash2", "crash3", "crash4", "crash5":
		nextFws.add(cd.args[1:])
	case "add":
		nextFws.add(cd.args[1:])
	case "del":
		nextFws.del(cd.args[1:])
	case "excl":
		nextFws.del(cd.args[1:])
	case "restart":
		return
	default:
		log.Fatalf("Unknown command %v\n", cd.args[0])
	}

	cd.nextConf = balance(cd.conf, nextFws)

	db.DLPrintf("COORD", "Coord conf %v next conf: %v %v\n", cd.conf,
		cd.nextConf, nextFws)

	// A gracefully exiting KV must ack too. We add it back to followers
	// after balance() without it.
	if cd.args[0] == "del" {
		nextFws.add(cd.args[1:])

	}

	if cd.args[0] == "crash1" {
		db.DLPrintf("COORD", "Crash1\n")
		os.Exit(1)
	}

	cd.Remove(KVCONFIGTMP) // don't care if succeeds or not
	cd.rmStatusFiles(KVPREPARED)
	cd.rmStatusFiles(KVCOMMITTED)

	nextFws.setKVWatches(cd.watchKV)

	log.Printf("COORD prepare\n")

	ok, n := cd.prepare(nextFws)

	log.Printf("COORD commit/abort %v\n", ok)

	if ok && cd.args[0] == "excl" {
		// make empty shards for the ones we lost; if it fails,
		// abort 2PC.
		ok = cd.initShards(cd.args[1:])
	}

	cd.commit(nextFws, nextFws.len()-n, ok)

	cd.Exit()
}
