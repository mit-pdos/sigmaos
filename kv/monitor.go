package kv

import (
	"log"
	"strconv"
	"sync"

	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	np "ulambda/ninep"
	"ulambda/procclnt"
	"ulambda/stats"
)

const (
	MAXLOAD float64 = 85.0
	MINLOAD float64 = 40.0
)

type Monitor struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	group int
}

func MakeMonitor(fslib *fslib.FsLib, pclnt *procclnt.ProcClnt) *Monitor {
	mo := &Monitor{}
	mo.FsLib = fslib
	mo.ProcClnt = pclnt
	return mo
}

func SpawnGrp(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, grp string) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, 3, "bin/user/kvd", []string{grp}, 2, 0)
}

func (mo *Monitor) grow() {
	SpawnGrp(mo.FsLib, mo.ProcClnt, strconv.Itoa(mo.group))
	BalancerOp(mo.FsLib, "add", group.GRP+strconv.Itoa(mo.group))
}

func (mo *Monitor) shrink(kv string) {
	BalancerOp(mo.FsLib, "del", kv)
	n := np.MEMFS + "/" + kv + "/"
	err := mo.Evict(kv)
	if err != nil {
		log.Printf("shrink: remove %v failed %v\n", n, err)
	}
}

// XXX Use load too?
func (mo *Monitor) doMonitor(conf *Config) {
	kvs := makeKvs(conf.Shards)
	log.Printf("Monitor config %v\n", kvs)

	util := float64(0)
	low := float64(100.0)
	lowkv := ""
	var lowload stats.Tload
	n := 0
	for kv, _ := range kvs.set {
		kvd := np.MEMFS + "/" + kv + "/statsd"
		sti := stats.StatInfo{}
		err := mo.GetFileJson(kvd, &sti)
		if err != nil {
			log.Printf("ReadFileJson %v failed %v\n", kvd, err)
		}
		n += 1
		util += sti.Util
		if sti.Util < low {
			low = sti.Util
			lowkv = kv
			lowload = sti.Load
		}
		log.Printf("path %v\n", sti.SortPath())
	}
	util = util / float64(n)
	log.Printf("monitor: avg util %.1f low %.1f kv %v %v\n", util, low, lowkv, lowload)
	if util >= MAXLOAD {
		mo.grow()
	}
	if util < MINLOAD && len(kvs.set) > 1 {
		mo.shrink(lowkv)
	}
}
