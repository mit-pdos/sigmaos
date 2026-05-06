package imgrec_py

import (
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

type ImgrecBlinkJob struct {
	conf *ImgrecPyJobConfig
	*sigmaclnt.SigmaClnt
}

func NewImgrecBlinkJob(conf *ImgrecPyJobConfig, sc *sigmaclnt.SigmaClnt) *ImgrecBlinkJob {
	return &ImgrecBlinkJob{conf: conf, SigmaClnt: sc}
}

func (j *ImgrecBlinkJob) Run(sigmaPath string) (string, error) {
	asyncFetchStr := "0"
	if j.conf.AsyncFetch {
		asyncFetchStr = "1"
	}
	p := proc.NewProc("imgrec-blink.py", []string{
		j.conf.ImgBucket, j.conf.ImgKey,
		j.conf.ModelBucket, j.conf.ModelKey,
		j.conf.Kid,
		asyncFetchStr,
	})
	p.GetProcEnv().UseSPProxy = true
	p.GetProcEnv().UseSPProxyProcClnt = true
	p.SetProcContainerType(proc.ProcContainerType_PROC_CTR_BLINK)
	if j.conf.Mcpu > 0 {
		p.SetMcpu(j.conf.Mcpu)
	}
	if j.conf.ShmemMB > 0 {
		p.SetShmemMB(j.conf.ShmemMB)
	}
	return spawnAndWait(j.SigmaClnt, p, sigmaPath)
}

