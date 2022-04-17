package kv

import (
	"log"
	"strconv"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	np "ulambda/ninep"
	"ulambda/procclnt"
	"ulambda/stats"
)

const (
	MAXLOAD        float64 = 85.0
	MINLOAD        float64 = 40.0
	CRASHKVD               = 40000
	KVD_NO_REPL    int     = 0
	KVD_REPL_LEVEL         = 3
)

type Monitor struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	group int
	grps  map[string]*groupmgr.GroupMgr
}

func MakeMonitor(fslib *fslib.FsLib, pclnt *procclnt.ProcClnt) *Monitor {
	mo := &Monitor{}
	mo.FsLib = fslib
	mo.ProcClnt = pclnt
	mo.group = 1
	return mo
}

func (mo *Monitor) nextGroup() string {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	gn := strconv.Itoa(mo.group)
	mo.group += 1
	return gn

}

func (mo *Monitor) rmgrp(gn string) (*groupmgr.GroupMgr, bool) {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	if grp, ok := mo.grps[gn]; ok {
		delete(mo.grps, gn)
		return grp, true
	} else {
		return nil, false
	}
}

func SpawnGrp(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, grp string, repl, ncrash int) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, repl, "bin/user/kvd", []string{grp}, ncrash, CRASHKVD, 0, 0)
}

func (mo *Monitor) grow() {
	gn := mo.nextGroup()
	db.DPrintf("KVMON", "Add group %v\n", gn)
	grp := SpawnGrp(mo.FsLib, mo.ProcClnt, gn, KVD_NO_REPL, 0)
	err := BalancerOp(mo.FsLib, "add", group.GRP+strconv.Itoa(mo.group))
	if err != nil {
		grp.Stop()
	}
	mo.mu.Lock()
	mo.grps[gn] = grp
	mo.mu.Unlock()
}

func (mo *Monitor) shrink(gn string) {
	db.DPrintf("KVMON", "Del group %v\n", gn)
	grp, ok := mo.rmgrp(gn)
	if !ok {
		db.DFatalf("rmgrp %v failed\n", gn)
	}
	err := BalancerOp(mo.FsLib, "del", gn)
	if err != nil {
		db.DPrintf("KVMON", "Del group %v failed\n", gn)
	}
	grp.Stop()
}

// XXX Use load too?
func (mo *Monitor) doMonitor(conf *Config) {
	kvs := makeKvs(conf.Shards)
	db.DPrintf(db.ALWAYS, "Monitor config %v\n", kvs)

	util := float64(0)
	low := float64(100.0)
	lowkv := ""
	var lowload stats.Tload
	n := 0
	for gn, _ := range kvs.set {
		kvd := group.GRPDIR + "/" + gn + "/" + np.STATSD
		sti := stats.StatInfo{}
		err := mo.GetFileJson(kvd, &sti)
		if err != nil {
			db.DPrintf(db.ALWAYS, "ReadFileJson %v failed %v\n", kvd, err)
		}
		n += 1
		util += sti.Util
		if sti.Util < low {
			low = sti.Util
			lowkv = gn
			lowload = sti.Load
		}
		log.Printf("path %v\n", sti.SortPath())
	}
	util = util / float64(n)
	db.DPrintf(db.ALWAYS, "monitor: avg util %.1f low %.1f kv %v %v\n", util, low, lowkv, lowload)
	if util >= MAXLOAD {
		mo.grow()
	}
	if util < MINLOAD && len(kvs.set) > 1 {
		mo.shrink(lowkv)
	}
}
