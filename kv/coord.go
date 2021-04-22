package kv

//
// Shard coordinator: assigns shards to KVs.  Assumes no KV failures
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
	sh := &Coord{}
	sh.pid = args[0]
	sh.args = args[1:]
	sh.ch = make(chan Tstatus)

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
	sh.FsLibSrv = fls
	sh.Started(sh.pid)
	return sh, nil
}

func (sh *Coord) Exit() {
	sh.ExitFs(COORD)
}

func (sh *Coord) unlock() {
	log.Printf("COORD unlock\n")
	if err := sh.UnlockFile(KVDIR, KVLOCK); err != nil {
		log.Fatalf("Unlock failed failed %v\n", err)
	}
}

func (sh *Coord) readStatus(dir string) *Followers {
	sts, err := sh.ReadDir(dir)
	if err != nil {
		return nil
	}
	fw := mkFollowers(sh.FsLibSrv.Clnt(), nil)
	for _, st := range sts {
		fw.kvs[st.Name] = true
	}
	return fw
}

func (sh *Coord) doCommit(nextConf *Config, prepared *Followers) bool {
	kvds := mkFollowers(sh.FsLibSrv.Clnt(), nextConf.Shards)
	if prepared == nil || prepared.len() != kvds.len() {
		return false
	}
	for kv, _ := range kvds.kvs {
		if _, ok := prepared.kvs[kv]; !ok {
			return false
		}
	}
	return true
}

func (sh *Coord) restart() {
	sh.conf = readConfig(sh.FsLibSrv, KVCONFIG)

	if sh.nextConf == nil {
		// either commit/aborted or never started
		db.DLPrintf("COORD", "Restart: clean\n")
		return
	}

	sh.nextConf = readConfig(sh.FsLibSrv, KVNEXTCONFIG)
	prepared := sh.readStatus(KVPREPARED)
	committed := sh.readStatus(KVCOMMITTED)

	db.DLPrintf("COORD", "Restart: conf %v next %v prepared %v commit %v\n",
		sh.conf, sh.nextConf, prepared, committed)

	fws := mkFollowers(sh.FsLibSrv.Clnt(), sh.nextConf.Shards)
	if sh.doCommit(sh.nextConf, prepared) {
		db.DLPrintf("COORD", "Restart: finish commit %d\n", committed.len())
		sh.commit(fws, committed.len(), true)
	} else {
		db.DLPrintf("COORD", "Restart: abort\n")
		fws := mkFollowers(sh.FsLibSrv.Clnt(), sh.conf.Shards)
		sh.commit(fws, committed.len(), false)
	}
}

func (sh *Coord) initShards(exclKvs []string) bool {
	db.DLPrintf("COORD", "initShards %v\n", exclKvs)
	excl := make(map[string]bool)
	for _, kv := range exclKvs {
		excl[kv] = true
	}
	for s, kv := range sh.conf.Shards {
		if _, ok := excl[kv]; ok { // shard s has been lost
			kvd := sh.nextConf.Shards[s]
			dst := shardPath(kvd, s, sh.nextConf.N)
			db.DLPrintf("COORD: Init shard dir %v\n", dst)
			err := sh.Mkdir(dst, 0777)
			if err != nil {
				db.DLPrintf("KV", "initShards: mkdir %v err %v\n", dst, err)
				return false
			}
		}
	}
	return true
}

func (sh *Coord) rmStatusFiles(dir string) {
	sts, err := sh.ReadDir(dir)
	if err != nil {
		log.Fatalf("COORD: ReadDir commit error %v\n", err)
	}
	for _, st := range sts {
		fn := dir + st.Name
		err = sh.Remove(fn)
		if err != nil {
			db.DLPrintf("COORD", "Remove %v failed %v\n", fn, err)
		}
	}
}

func (sh *Coord) watchStatus(p string, err error) {
	db.DLPrintf("COORD", "watchStatus %v\n", p)
	status := ABORT
	b, err := sh.ReadFile(p)
	if err != nil {
		db.DLPrintf("COORD", "watchStatus ReadFile %v err %v\n", p, b)
	}
	if string(b) == "OK" {
		status = COMMIT
	}
	sh.ch <- status
}

func (sh *Coord) watchKV(p string, err error) {
	db.DLPrintf("COORD", "watchKV %v\n", p)
	sh.ch <- CRASH
}

func (sh *Coord) prepare(nextFws *Followers) (bool, int) {
	nextFws.setStatusWatches(KVPREPARED, sh.watchStatus)

	err := sh.MakeFileJsonAtomic(KVNEXTCONFIG, 0777, *sh.nextConf)
	if err != nil {
		db.DLPrintf("COORD", "COORD: MakeFileJsonAtomic %v err %v\n",
			KVNEXTCONFIG, err)
	}

	// depending how many KVs ack, crash2 results
	// in a abort or commit
	if sh.args[0] == "crash2" {
		db.DLPrintf("COORD", "Crash2\n")
		os.Exit(1)
	}

	success := true
	n := 0
	for i := 0; i < nextFws.len(); i++ {
		status := <-sh.ch
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

func (sh *Coord) commit(fws *Followers, ndone int, ok bool) {
	if ok {
		db.DLPrintf("COORD", "Commit to %v\n", sh.nextConf)
	} else {
		// Rename KVCONFIGTMP into KVNEXTCONFIG so that the followers
		// will abort to the old KVCONFIG
		if err := sh.CopyFile(KVCONFIG, KVCONFIGTMP); err != nil {
			db.DLPrintf("COORD", "CopyFile failed %v\n", err)
		}
		err := sh.Rename(KVCONFIGTMP, KVNEXTCONFIG)
		if err != nil {
			db.DLPrintf("COORD", "COORD: rename %v -> %v: error %v\n",
				KVCONFIGTMP, KVNEXTCONFIG, err)
			return
		}
		db.DLPrintf("COORD", "Abort to %v\n", sh.conf)
	}

	fws.setStatusWatches(KVCOMMITTED, sh.watchStatus)

	// commit/abort to new KVCONFIG, which maybe the same as the
	// old one
	err := sh.Rename(KVNEXTCONFIG, KVCONFIG)
	if err != nil {
		db.DLPrintf("COORD", "COORD: rename %v -> %v: error %v\n",
			KVNEXTCONFIG, KVCONFIG, err)
		return
	}

	// crash3 should results in commit (assuming no KVs crash)
	if sh.args[0] == "crash3" {
		db.DLPrintf("COORD", "Crash3\n")
		os.Exit(1)
	}

	for i := 0; i < fws.len()-ndone; i++ {
		s := <-sh.ch
		db.DLPrintf("COORD", "KV commit status %v\n", s)
	}

	db.DLPrintf("COORD", "Done commit/abort\n")
}

func (sh *Coord) TwoPC() {
	defer sh.unlock() // release lock acquired in MakeCoord()

	// db.DLPrintf("COORD", "Coord: %v\n", sh.args)
	log.Printf("COORD Coord: %v\n", sh.args)

	// XXX set removeWatch on KVs? maybe in KV

	sh.restart()

	// We may have committed/aborted; reread sh.conf to get new
	// this config
	sh.conf = readConfig(sh.FsLibSrv, KVCONFIG)

	nextFws := mkFollowers(sh.FsLibSrv.Clnt(), sh.conf.Shards)

	switch sh.args[0] {
	case "crash1", "crash2", "crash3", "crash4", "crash5":
		nextFws.add(sh.args[1:])
	case "add":
		nextFws.add(sh.args[1:])
	case "del":
		nextFws.del(sh.args[1:])
	case "excl":
		nextFws.del(sh.args[1:])
	case "restart":
		return
	default:
		log.Fatalf("Unknown command %v\n", sh.args[0])
	}

	sh.nextConf = balance(sh.conf, nextFws)

	db.DLPrintf("COORD", "Coord conf %v next conf: %v %v\n", sh.conf,
		sh.nextConf, nextFws)

	// A gracefully exiting KV must ack too. We add it back to followers
	// after balance() without it.
	if sh.args[0] == "del" {
		nextFws.add(sh.args[1:])

	}

	if sh.args[0] == "crash1" {
		db.DLPrintf("COORD", "Crash1\n")
		os.Exit(1)
	}

	sh.Remove(KVCONFIGTMP) // don't care if succeeds or not
	sh.rmStatusFiles(KVPREPARED)
	sh.rmStatusFiles(KVCOMMITTED)

	nextFws.setKVWatches(sh.watchKV)

	log.Printf("COORD prepare\n")

	ok, n := sh.prepare(nextFws)

	log.Printf("COORD commit/abort %v\n", ok)

	if ok && sh.args[0] == "excl" {
		// make empty shards for the ones we lost; if it fails
		// abort 2PC.
		ok = sh.initShards(sh.args[1:])
	}

	sh.commit(nextFws, nextFws.len()-n, ok)

	sh.Exit()
}
