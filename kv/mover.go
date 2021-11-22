package kv

import (
	"fmt"
	"log"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	"ulambda/proc"
	"ulambda/procclnt"
)

type Mover struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	shard string
	src   string
	dst   string
}

func MakeMover(args []string) (*Mover, error) {
	mv := &Mover{}
	if len(args) != 3 {
		return nil, fmt.Errorf("MakeMover: too few arguments %v\n", args)
	}
	mv.shard = args[0]
	mv.src = args[1]
	mv.dst = args[2]
	mv.FsLib = fslib.MakeFsLib(proc.GetPid())
	mv.ProcClnt = procclnt.MakeProcClnt(mv.FsLib)

	db.Name(proc.GetPid())

	mv.Started(proc.GetPid())
	return mv, nil
}

func shardDir(kvd string) string {
	return named.MEMFS + "/" + kvd
}

func shardPath(kvd, shard string) string {
	return named.MEMFS + "/" + kvd + "/shard" + shard
}

func keyPath(kvd, shard string, k string) string {
	d := shardPath(kvd, shard)
	return d + "/" + k
}

func shardTmp(shardp string) string {
	return shardp + "#"
}

// Move shard from src to dst
func (mv *Mover) moveShard(shard, src, dst string) error {
	s := shardPath(src, shard)
	s = shardTmp(s)
	d := shardPath(dst, shard)
	d1 := shardTmp(d)
	err := mv.Mkdir(d1, 0777)
	// an aborted view change may have created the directory
	if err != nil && !strings.HasPrefix(err.Error(), "Name exists") {
		return err
	}
	db.DLPrintf("MV", "Copy shard from %v to %v\n", s, d1)
	err = mv.CopyDir(s, d1)
	if err != nil {
		return err
	}
	db.DLPrintf("MV", "Copy shard from %v to %v done\n", s, d1)
	err = mv.Rename(d1, d)
	if err != nil {
		return err
	}
	return nil
}

func (mv *Mover) removeShard(shard, src string) {
	d := shardPath(src, shard)
	d = shardTmp(d)
	mv.RmDir(d)
}

// func (mv *Mover) closeFid(shard string) {
// 	db.DLPrintf("MV", "closeFids shard %v\n", shard)
// 	mv.ConnTable().IterateFids(func(f *npo.Fid) {
// 		p1 := np.Join(f.Path())
// 		uname := f.Ctx().Uname()
// 		if strings.HasPrefix(uname, "clerk") && strings.HasPrefix(p1, shard) {
// 			db.DLPrintf("MV", "CloseFid: mark closed %v %v\n", uname, p1)
// 			f.Close()
// 		}
// 	})
// }

// // Close fids for which i will not be responsible, signaling to
// // clients to failover to another server.
// func (mv *Mover) closeFids() {
// 	for s, kvd := range mv.conf2.Shards {
// 		if kvd != mv.kv && mv.conf.Shards[s] == mv.kv {
// 			mv.closeFid("shard" + strconv.Itoa(s))
// 		}
// 	}
// }

func (mv *Mover) Work() {
	log.Printf("MV shard %v from %v to %v\n", mv.shard, mv.src, mv.dst)
	if err := mv.moveShard(mv.shard, mv.src, mv.dst); err != nil {
		log.Printf("MV moveShards %v %v err %v\n", mv.src, mv.dst, err)
	}

	mv.removeShard(mv.shard, mv.src)
}

func (mv *Mover) Exit() {
	mv.Exited(proc.GetPid(), "OK")
}
