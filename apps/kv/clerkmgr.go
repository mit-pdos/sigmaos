package kv

import (
	"fmt"
	"path/filepath"
	"strconv"

	proto "sigmaos/apps/cache/proto"

	"sigmaos/apps/kv/kvgrp"
	"sigmaos/apps/cache"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type ClerkMgr struct {
	*KvClerk
	*sigmaclnt.SigmaClnt
	job     string
	sempath string
	sem     *semclnt.SemClnt
	ckmcpu  proc.Tmcpu // Number of exclusive cores allocated to each clerk.
	clrks   []sp.Tpid
	repl    bool
}

func NewClerkMgr(sc *sigmaclnt.SigmaClnt, job string, mcpu proc.Tmcpu, repl bool) (*ClerkMgr, error) {
	cm := &ClerkMgr{SigmaClnt: sc, job: job, ckmcpu: mcpu, repl: repl}
	clrk := NewClerkFsLib(cm.SigmaClnt.FsLib, cm.job, repl)
	cm.KvClerk = clrk
	cm.sempath = filepath.Join(kvgrp.JobDir(job), "kvclerk-sem")
	cm.sem = semclnt.NewSemClnt(sc.FsLib, cm.sempath)
	if err := cm.sem.Init(0); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cm *ClerkMgr) Nclerk() int {
	return len(cm.clrks)
}

func (cm *ClerkMgr) StartCmClerk() error {
	return cm.StartClerk()
}

func (cm *ClerkMgr) InitKeys(nkeys int) error {
	for i := uint64(0); i < uint64(nkeys); i++ {
		if err := cm.Put(cache.NewKey(i), &proto.CacheString{Val: ""}); err != nil {
			return err
		}
	}
	return nil
}

func (cm *ClerkMgr) StartClerks(dur string, nclerk int) error {
	return cm.AddClerks(dur, nclerk)
}

// Add or remove clerk clerks
func (cm *ClerkMgr) AddClerks(dur string, nclerk int) error {
	if nclerk == 0 {
		return nil
	}
	var ck sp.Tpid
	if nclerk < 0 {
		for ; nclerk < 0; nclerk++ {
			ck, cm.clrks = cm.clrks[0], cm.clrks[1:]
			_, err := cm.stopClerk(ck)
			if err != nil {
				return err
			}
		}
	} else {
		for ; nclerk > 0; nclerk-- {
			ck, err := cm.startClerk(dur, cm.ckmcpu)
			if err != nil {
				return err
			}
			cm.clrks = append(cm.clrks, ck)
		}
		cm.sem.Up()
	}
	return nil
}

func (cm *ClerkMgr) StopClerks() error {
	db.DPrintf(db.ALWAYS, "clerks to evict %v\n", len(cm.clrks))
	for _, ck := range cm.clrks {
		status, err := cm.stopClerk(ck)
		if err != nil {
			return err
		}
		db.DPrintf(db.ALWAYS, "Clerk exit status %v\n", status)
		if !(status.IsStatusEvicted() || status.IsStatusOK()) {
			return fmt.Errorf("wrong status %v", status)
		}
	}
	return nil
}

func (cm *ClerkMgr) WaitForClerks() error {
	for _, ck := range cm.clrks {
		status, err := cm.WaitExit(ck)
		if err != nil {
			return err
		}
		if !status.IsStatusOK() {
			return fmt.Errorf("clerk exit err %v\n", status)
		}
		db.DPrintf(db.ALWAYS, "Clerk %v ops/s\n", status.Data().(float64))
	}
	return nil
}

func (cm *ClerkMgr) startClerk(dur string, mcpu proc.Tmcpu) (sp.Tpid, error) {
	idx := len(cm.clrks)
	var args []string
	if dur != "" {
		args = []string{dur, strconv.Itoa(idx * NKEYS), cm.sempath}
	}
	repl := ""
	if cm.repl {
		repl = "repl"
	}
	args = append([]string{cm.job, repl}, args...)
	p := proc.NewProc("kv-clerk", args)
	p.SetMcpu(mcpu)

	if err := cm.Spawn(p); err != nil {
		return p.GetPid(), err
	}
	err := cm.WaitStart(p.GetPid())
	return p.GetPid(), err
}

func (cm *ClerkMgr) stopClerk(pid sp.Tpid) (*proc.Status, error) {
	err := cm.Evict(pid)
	if err != nil {
		return nil, err
	}
	status, err := cm.WaitExit(pid)
	return status, err
}

func (cm *ClerkMgr) GetKeyCountsPerGroup(nkeys int) map[string]int {
	keys := make([]string, 0, nkeys)
	for i := uint64(0); i < uint64(nkeys); i++ {
		keys = append(keys, cache.NewKey(i))
	}
	return cm.KvClerk.GetKeyCountsPerGroup(keys)
}
