package imgrec_py

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/proc"
	wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const cosandboxName = "imgrec_boot"

type ImgrecPyJobConfig struct {
	ImgBucket    string     `json:"img_bucket"`
	ImgKey       string     `json:"img_key"`
	ModelBucket  string     `json:"model_bucket"`
	ModelKey     string     `json:"model_key"`
	Kid          string     `json:"kid"`
	UseCoSandbox bool       `json:"use_co_sandbox"`
	AsyncFetch   bool       `json:"async_fetch"`
	ShmemMB      proc.Tmem  `json:"shmem_mb"`
	Mcpu         proc.Tmcpu `json:"mcpu"`
}

func NewImgrecPyJobConfig(imgBucket, imgKey, modelBucket, modelKey, kid string, useCoSandbox bool, asyncFetch bool, shmemMB proc.Tmem, mcpu proc.Tmcpu) *ImgrecPyJobConfig {
	return &ImgrecPyJobConfig{
		ImgBucket:    imgBucket,
		ImgKey:       imgKey,
		ModelBucket:  modelBucket,
		ModelKey:     modelKey,
		Kid:          kid,
		UseCoSandbox: useCoSandbox,
		AsyncFetch:   asyncFetch,
		ShmemMB:      shmemMB,
		Mcpu:         mcpu,
	}
}

type ImgrecPyJob struct {
	conf *ImgrecPyJobConfig
	*sigmaclnt.SigmaClnt
	coSandbox []byte
	bootInput []byte
}

func NewImgrecPyJob(conf *ImgrecPyJobConfig, sc *sigmaclnt.SigmaClnt) (*ImgrecPyJob, error) {
	j := &ImgrecPyJob{conf: conf, SigmaClnt: sc}
	if conf.UseCoSandbox {
		b, err := wasmrt.ReadCoSandbox(sc, cosandboxName)
		if err != nil {
			db.DPrintf(db.ERROR, "ImgrecPy ReadCoSandbox err: %v", err)
			return nil, err
		}
		j.coSandbox = b
		j.bootInput = wasmrt.EncodeArgs([]string{conf.ImgBucket, conf.ImgKey, conf.ModelBucket, conf.ModelKey, conf.Kid})
	}
	return j, nil
}

// Run spawns an imgrec.py proc, waits for it to complete, and returns the
// result message (class_idx,score).
func (j *ImgrecPyJob) Run(sigmaPath string) (string, error) {
	asyncFetchStr := "0"
	if j.conf.AsyncFetch {
		asyncFetchStr = "1"
	}
	p := proc.NewProc("imgrec.py", []string{
		j.conf.ImgBucket, j.conf.ImgKey,
		j.conf.ModelBucket, j.conf.ModelKey,
		j.conf.Kid,
		asyncFetchStr,
	})
	p.GetProcEnv().UseSPProxy = true
	p.GetProcEnv().UseSPProxyProcClnt = true
	p.SetProcContainerType(proc.ProcContainerType_PROC_CTR_PYTHON)
	if j.conf.Mcpu > 0 {
		p.SetMcpu(j.conf.Mcpu)
	}
	if j.conf.UseCoSandbox {
		p.SetCoSandbox(j.coSandbox, j.bootInput)
		p.SetRunCoSandbox(true)
	}
	if j.conf.ShmemMB > 0 {
		p.SetShmemMB(j.conf.ShmemMB)
	}
	db.DPrintf(db.TEST, "Scale %v", p.GetPid())
	return spawnAndWait(j.SigmaClnt, p, sigmaPath)
}

func spawnAndWait(sc *sigmaclnt.SigmaClnt, p *proc.Proc, sigmaPath string) (string, error) {
	if sigmaPath != sp.NOT_SET {
		p.PrependSigmaPath(sigmaPath)
	}
	if err := sc.Spawn(p); err != nil {
		db.DPrintf(db.ERROR, "Spawn err: %v", err)
		return "", err
	}
	if err := sc.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.ERROR, "WaitStart err: %v", err)
		return "", err
	}
	status, err := sc.WaitExit(p.GetPid())
	if err != nil {
		db.DPrintf(db.ERROR, "WaitExit err: %v", err)
		return "", err
	}
	if !status.IsStatusOK() {
		return "", fmt.Errorf("proc exited with status: %v", status)
	}
	return status.Msg(), nil
}
