package clnt

import (
	"fmt"
	"strconv"
	"time"

	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func StartClerk(sc *sigmaclnt.SigmaClnt, job string, nkeys int, dur time.Duration, keyOffset int, sempn string, mcpu proc.Tmcpu) (sp.Tpid, error) {
	p := proc.NewProc("cachedsvc-clerk", []string{job, strconv.Itoa(nkeys), dur.String(), strconv.Itoa(keyOffset), sempn})
	p.SetMcpu(mcpu)
	err := sc.Spawn(p)
	if err != nil {
		return "", err
	}
	err = sc.WaitStart(p.GetPid())
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
