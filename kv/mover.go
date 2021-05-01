package kv

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

type Mover struct {
	mu sync.Mutex
	*fslib.FsLib
	pid   string
	kv    string
	args  []string
	conf2 *Config2
}

func MakeMover(args []string) (*Mover, error) {
	mv := &Mover{}
	if len(args) != 2 {
		return nil, fmt.Errorf("MakeMover: too few arguments %v\n", args)
	}
	log.Printf("MakeMover %v\n", args[1])
	mv.pid = args[0]
	mv.kv = args[1]
	mv.FsLib = fslib.MakeFsLib(mv.pid)

	db.Name(mv.pid)

	mv.Started(mv.pid)
	return mv, nil
}

func shardDir(kvd string) string {
	return memfsd.MEMFS + "/" + kvd
}

func shardPath(kvd string, shard, view int) string {
	return memfsd.MEMFS + "/" + kvd + "/shard" + strconv.Itoa(shard) + "-" + strconv.Itoa(view)
}

func keyPath(kvd string, shard int, view int, k string) string {
	d := shardPath(kvd, shard, view)
	return d + "/" + k
}

func shardTmp(shardp string) string {
	return shardp + "#"
}

// Move shard: either copy to new shard server or rename shard dir
// for new view.
func (mv *Mover) moveShard(s int, kvd string) error {
	src := shardPath(mv.kv, s, mv.conf2.N-1)
	src = shardTmp(src)
	if kvd != mv.kv { // Copy
		dst := shardPath(kvd, s, mv.conf2.N)
		err := mv.Mkdir(dst, 0777)
		// an aborted view change may have created the directory
		if err != nil && !strings.HasPrefix(err.Error(), "Name exists") {
			return err
		}
		db.DLPrintf("MV", "Copy shard from %v to %v\n", src, dst)
		err = mv.CopyDir(src, dst)
		if err != nil {
			return err
		}
		db.DLPrintf("MV", "Copy shard from %v to %v done\n", src, dst)
	} else { // rename
		dst := shardPath(kvd, s, mv.conf2.N)
		err := mv.Rename(src, dst)
		if err != nil {
			log.Printf("MV Rename %v -> %v failed %v\n", src, dst, err)
		}
	}
	return nil
}

func (mv *Mover) moveShards() error {
	for s, kvd := range mv.conf2.Old {
		if kvd == mv.kv && mv.conf2.New[s] != "" {
			if err := mv.moveShard(s, mv.conf2.New[s]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mv *Mover) removeShards(version int) {
	d := shardDir(mv.kv)
	mv.ProcessDir(d, func(st *np.Stat) (bool, error) {
		name := strings.Trim(st.Name, "#")
		splits := strings.Split(name, "-")
		n, err := strconv.Atoi(splits[1])
		if err != nil {
			return false, nil
		}
		if n < 0 || n > version {
			return false, nil
		}
		d := d + "/" + st.Name
		db.DLPrintf("MV", "RmDir shard %v\n", d)
		err = mv.RmDir(d)
		if err != nil {
			log.Printf("MV remove %v err %v\n", d, err)
		}
		return false, nil
	})
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
	var err error
	mv.conf2, err = readConfig2(mv.FsLib, KVNEXTCONFIG)
	if err != nil {
		log.Fatalf("MV: read %v err %v\n", KVCONFIG, err)
	}

	// db.DLPrintf("MV", "change %v\n", mv.conf2)
	log.Printf("MV change %v\n", mv.conf2)

	if err := mv.moveShards(); err != nil {
		log.Printf("MV moveShards %v err %v\n", mv.kv, err)
	}

	mv.removeShards(mv.conf2.N - 2)
}
