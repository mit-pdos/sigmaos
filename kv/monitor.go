package kv

import (
	"strconv"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
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

type grpMap struct {
	sync.Mutex
	grps map[string]*groupmgr.GroupMgr
}

func mkGrpMap() *grpMap {
	gm := &grpMap{}
	gm.grps = make(map[string]*groupmgr.GroupMgr)
	return gm
}

func (gm *grpMap) insert(gn string, grp *groupmgr.GroupMgr) {
	gm.Lock()
	defer gm.Unlock()
	gm.grps[gn] = grp
}

func (gm *grpMap) delete(gn string) (*groupmgr.GroupMgr, bool) {
	gm.Lock()
	defer gm.Unlock()
	if grp, ok := gm.grps[gn]; ok {
		delete(gm.grps, gn)
		return grp, true
	} else {
		return nil, false
	}
}

func (gm *grpMap) groups() []*groupmgr.GroupMgr {
	gm.Lock()
	defer gm.Unlock()
	gs := make([]*groupmgr.GroupMgr, 0, len(gm.grps))
	for _, grp := range gm.grps {
		gs = append(gs, grp)
	}
	return gs
}

type Monitor struct {
	*fslib.FsLib
	*procclnt.ProcClnt

	mu       sync.Mutex
	group    int
	kvdncore proc.Tcore
	gm       *grpMap
}

func MakeMonitor(fslib *fslib.FsLib, pclnt *procclnt.ProcClnt, kvdncore proc.Tcore) *Monitor {
	mo := &Monitor{}
	mo.FsLib = fslib
	mo.ProcClnt = pclnt
	mo.group = 1
	mo.kvdncore = kvdncore
	mo.gm = mkGrpMap()
	return mo
}

func (mo *Monitor) nextGroup() string {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	gn := strconv.Itoa(mo.group)
	mo.group += 1
	return group.GRP + gn
}

func (mo *Monitor) grow() {
	gn := mo.nextGroup()
	db.DPrintf("KVMON", "Add group %v\n", gn)
	grp := SpawnGrp(mo.FsLib, mo.ProcClnt, gn, mo.kvdncore, KVD_NO_REPL, 0)
	err := BalancerOp(mo.FsLib, "add", gn)
	if err != nil {
		grp.Stop()
	}
	mo.gm.insert(gn, grp)
}

func (mo *Monitor) shrink(gn string) {
	db.DPrintf("KVMON", "Del group %v\n", gn)
	grp, ok := mo.gm.delete(gn)
	if !ok {
		db.DFatalf("rmgrp %v failed\n", gn)
	}
	err := BalancerOp(mo.FsLib, "del", gn)
	if err != nil {
		db.DPrintf("KVMON", "Del group %v failed\n", gn)
	}
	grp.Stop()
}

func (mo *Monitor) done() {
	db.DPrintf("KVMON", "shutdown groups\n")
	for _, grp := range mo.gm.groups() {
		grp.Stop()
	}
}

// XXX Use load too?
func (mo *Monitor) doMonitor(conf *Config) {
	kvs := MakeKvs(conf.Shards)
	db.DPrintf(db.ALWAYS, "Monitor config %v\n", kvs)

	util := float64(0)
	low := float64(100.0)
	lowkv := ""
	var lowload perf.Tload
	n := 0
	for gn, _ := range kvs.Set {
		kvgrp := group.GrpDir(gn) + np.STATSD
		sti := stats.StatInfo{}
		err := mo.GetFileJson(kvgrp, &sti)
		if err != nil {
			db.DPrintf(db.ALWAYS, "ReadFileJson %v failed %v\n", kvgrp, err)
		}
		n += 1
		util += sti.Util
		if sti.Util < low {
			low = sti.Util
			lowkv = gn
			lowload = sti.Load
		}
		// db.DPrintf("KVMON", "path %v\n", sti.SortPath())
	}
	util = util / float64(n)
	db.DPrintf(db.ALWAYS, "monitor: avg util %.1f low %.1f kv %v %v\n", util, low, lowkv, lowload)
	if util >= MAXLOAD {
		mo.grow()
	}
	if util < MINLOAD && len(kvs.Set) > 1 {
		mo.shrink(lowkv)
	}
}
