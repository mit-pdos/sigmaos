package cachedsvcclnt

import (
	"fmt"
	"strconv"
	"time"

	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func StartClerk(sc *sigmaclnt.SigmaClnt, job string, nkeys int, dur time.Duration, keyOffset int, sempn string, mcpu proc.Tmcpu) (sp.Tpid, error) {
	p := proc.MakeProc("cachedsvc-clerk", []string{job, strconv.Itoa(nkeys), dur.String(), strconv.Itoa(keyOffset), sempn})
	p.SetMcpu(mcpu)
	// SpawnBurst to spread clerks across procds.
	_, errs := sc.SpawnBurst([]*proc.Proc{p}, 2)
	if len(errs) > 0 {
		return "", errs[0]
	}
	err := sc.WaitStart(p.GetPid())
	if err != nil {
		return "", err
	}
	return p.GetPid(), nil
}

func WaitClerk(sc *sigmaclnt.SigmaClnt, pid sp.Tpid) (float64, error) {
	status, err := sc.WaitExit(pid)
	if err != nil {
		return 0.0, err
	}
	if !status.IsStatusOK() {
		return 0.0, fmt.Errorf("Bad status %v", status)
	}
	return status.Data().(float64), nil
}
