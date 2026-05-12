package imgrec_py

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
	"sigmaos/sigmaclnt"
)

const cosandboxUXName = "imgrec_ux_boot"

type ImgrecBlinkJob struct {
	conf      *ImgrecPyJobConfig
	coSandbox []byte
	bootInput []byte
	*sigmaclnt.SigmaClnt
}

func NewImgrecBlinkJob(conf *ImgrecPyJobConfig, sc *sigmaclnt.SigmaClnt) (*ImgrecBlinkJob, error) {
	j := &ImgrecBlinkJob{conf: conf, SigmaClnt: sc}
	if conf.UseCoSandbox {
		var sandboxName string
		var bootArgs []string
		if conf.ImgBucket == "ux" {
			sandboxName = cosandboxUXName
			modelPath := conf.ModelKey
			if conf.ModelLocalPath != "" {
				modelPath = ""
			}
			bootArgs = []string{conf.ImgKey, modelPath, conf.Kid}
		} else {
			sandboxName = cosandboxName
			modelBucket := conf.ModelBucket
			modelKey := conf.ModelKey
			if conf.ModelLocalPath != "" {
				modelBucket = ""
				modelKey = ""
			}
			bootArgs = []string{conf.ImgBucket, conf.ImgKey, modelBucket, modelKey, conf.Kid}
		}
		b, err := wasmrt.ReadCoSandbox(sc, sandboxName)
		if err != nil {
			db.DPrintf(db.ERROR, "ImgrecBlink ReadCoSandbox err: %v", err)
			return nil, err
		}
		j.coSandbox = b
		j.bootInput = wasmrt.EncodeArgs(bootArgs)
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
