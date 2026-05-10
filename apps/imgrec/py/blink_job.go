package imgrec_py

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
	"sigmaos/sigmaclnt"
)

type ImgrecBlinkJob struct {
	conf      *ImgrecPyJobConfig
	coSandbox []byte
	bootInput []byte
	*sigmaclnt.SigmaClnt
}

func NewImgrecBlinkJob(conf *ImgrecPyJobConfig, sc *sigmaclnt.SigmaClnt) (*ImgrecBlinkJob, error) {
	j := &ImgrecBlinkJob{conf: conf, SigmaClnt: sc}
	if conf.UseCoSandbox {
		b, err := wasmrt.ReadCoSandbox(sc, cosandboxName)
		if err != nil {
			db.DPrintf(db.ERROR, "ImgrecBlink ReadCoSandbox err: %v", err)
			return nil, err
		}
		j.coSandbox = b
		modelBucket := conf.ModelBucket
		modelKey := conf.ModelKey
		if conf.ModelLocalPath != "" {
			modelBucket = ""
			modelKey = ""
		}
		j.bootInput = wasmrt.EncodeArgs([]string{conf.ImgBucket, conf.ImgKey, modelBucket, modelKey, conf.Kid})
	}
	return j, nil
}

func (j *ImgrecBlinkJob) Run(sigmaPath string) (string, error) {
	asyncFetchStr := "0"
	if j.conf.AsyncFetch {
		asyncFetchStr = "1"
	}
	p := proc.NewProcPid("imgrec-blink-1", "imgrec-blink.py", []string{
		j.conf.ImgBucket, j.conf.ImgKey,
		j.conf.ModelBucket, j.conf.ModelKey,
		j.conf.Kid,
		asyncFetchStr,
		j.conf.ModelLocalPath,
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
	if j.conf.UseCoSandbox {
		p.SetCoSandbox(j.coSandbox, j.bootInput)
		p.SetRunCoSandbox(true)
	}
	return spawnAndWait(j.SigmaClnt, p, sigmaPath)
}
