package kv

import (
	"strconv"
	"sync"

	db "sigmaos/debug"
	"sigmaos/groupmgr"
	"sigmaos/kvgrp"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

//
// Adds or removes shards based on load
//

const (
	MAXLOAD        float64 = 85.0
	MINLOAD        float64 = 40.0
	CRASHKVD               = 5000
	KVD_NO_REPL    int     = 0
	KVD_REPL_LEVEL         = 3
)

type grpMap struct {
	sync.Mutex
	grps map[string]*groupmgr.GroupMgr
}

func newGrpMap() *grpMap {
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
	*sigmaclnt.SigmaClnt

	mu      sync.Mutex
	job     string
	group   int
	kvdmcpu proc.Tmcpu
	gm      *grpMap
}

func NewMonitor(sc *sigmaclnt.SigmaClnt, job string, kvdmcpu proc.Tmcpu) *Monitor {
	mo := &Monitor{}
	mo.SigmaClnt = sc
	mo.group = 1
	mo.job = job
	mo.kvdmcpu = kvdmcpu
	mo.gm = newGrpMap()
	return mo
}

func (mo *Monitor) nextGroup() string {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	gn := strconv.Itoa(mo.group)
	mo.group += 1
	return GRP + gn
}

func (mo *Monitor) grow() error {
	gn := mo.nextGroup()
	db.DPrintf(db.KVMON, "Add group %v\n", gn)
	grp, err := spawnGrp(mo.SigmaClnt, mo.job, gn, mo.kvdmcpu, KVD_NO_REPL, 0)
	if err != nil {
		return err
	}
	if err := BalancerOp(mo.FsLib, mo.job, "add", gn); err != nil {
		grp.StopGroup()
		return err
	}
	mo.gm.insert(gn, grp)
	return nil
}

func (mo *Monitor) shrink(gn string) {
	db.DPrintf(db.KVMON, "Del group %v\n", gn)
	grp, ok := mo.gm.delete(gn)
	if !ok {
		db.DFatalf("rmgrp %v failed\n", gn)
	}
	err := BalancerOp(mo.FsLib, mo.job, "del", gn)
	if err != nil {
		db.DPrintf(db.KVMON, "Del group %v failed\n", gn)
	}
	grp.StopGroup()
}

func (mo *Monitor) done() {
	db.DPrintf(db.KVMON, "shutdown groups\n")
	for _, grp := range mo.gm.groups() {
		grp.StopGroup()
	}
}

func (mo *Monitor) doMonitor(conf *Config) {
	kvs := NewKvs(conf.Shards)
	db.DPrintf(db.ALWAYS, "Monitor config %v\n", kvs)

	util := float64(0)
	low := float64(100.0)
	lowkv := ""
	var lowload perf.Tload
	n := 0
	for gn, _ := range kvs.Set {
		kvgrp := kvgrp.GrpPath(kvgrp.JobDir(mo.job), gn)
		st, err := mo.ReadStats(kvgrp)
		if err != nil {
			db.DPrintf(db.ALWAYS, "ReadStats %v failed %v\n", kvgrp, err)
		}
		db.DPrintf(db.KVMON, "%v: sti %v\n", kvgrp, st)
		n += 1
		util += st.Util
		if st.Util < low {
			low = st.Util
			lowkv = gn
			lowload = st.Load
		}
		// db.DPrintf(db.KVMON, "path %v\n", sti.SortPath())
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
