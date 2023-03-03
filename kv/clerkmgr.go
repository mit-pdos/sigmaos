package kv

import (
	"fmt"
	"path"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
)

type ClerkMgr struct {
	*KvClerk
	*sigmaclnt.SigmaClnt
	job     string
	sempath string
	sem     *semclnt.SemClnt
	nclerk  int
	clrks   []proc.Tpid
}

func MkClerkMgr(sc *sigmaclnt.SigmaClnt, job string, nclerk int) (*ClerkMgr, error) {
	cm := &ClerkMgr{SigmaClnt: sc, job: job, nclerk: nclerk}
	clrk, err := MakeClerkFsl(cm.SigmaClnt.FsLib, cm.job)
	if err != nil {
		return nil, err
	}
	cm.KvClerk = clrk
	cm.sempath = path.Join(JobDir(job), "kvclerk-sem")
	cm.sem = semclnt.MakeSemClnt(sc.FsLib, cm.sempath)
	if err := cm.sem.Init(0); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cm *ClerkMgr) InitKeys(nkeys int) error {
	for i := uint64(0); i < uint64(nkeys); i++ {
		if err := cm.PutRaw(MkKey(i), []byte{}, 0); err != nil {
			return err
		}
	}
	return nil
}

func (cm *ClerkMgr) StartClerks(dur string) error {
	for i := 0; i < cm.nclerk; i++ {
		var args []string
		if dur != "" {
			args = []string{dur, strconv.Itoa(i * NKEYS), cm.sempath}
		}
		pid, err := cm.startClerk(args, 0)
		if err != nil {
			return err
		}
		cm.clrks = append(cm.clrks, pid)
	}
	cm.sem.Up()
	return nil
}

func (cm *ClerkMgr) Stop() error {
	db.DPrintf(db.ALWAYS, "clerks to evict %v\n", len(cm.clrks))
	for _, ck := range cm.clrks {
		_, err := cm.stopClerk(ck)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cm *ClerkMgr) WaitForClerks() error {
	db.DPrintf(db.ALWAYS, "clerks to wait for %v\n", len(cm.clrks))
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

func (cm *ClerkMgr) startClerk(args []string, ncore proc.Tcore) (proc.Tpid, error) {
	args = append([]string{cm.job}, args...)
	p := proc.MakeProc("kv-clerk", args)
	p.SetNcore(ncore)
	// SpawnBurst to spread clerks across procds.
	_, errs := cm.SpawnBurst([]*proc.Proc{p}, 2)
	if len(errs) > 0 {
		return p.GetPid(), errs[0]
	}
	err := cm.WaitStart(p.GetPid())
	return p.GetPid(), err
}

func (cm *ClerkMgr) stopClerk(pid proc.Tpid) (*proc.Status, error) {
	err := cm.Evict(pid)
	if err != nil {
		return nil, err
	}
	status, err := cm.WaitExit(pid)
	return status, err
}
