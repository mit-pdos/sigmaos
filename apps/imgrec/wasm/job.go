package imgrec_wasm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	db "sigmaos/debug"
	"sigmaos/proc"
	wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	PrecompiledProg = "imgrec_precompiled.wasm"
	cosandboxName   = "imgrec_boot"
)

type ImgrecWASMJobConfig struct {
	ImgBucket         string     `json:"img_bucket"`
	ImgKey            string     `json:"img_key"`
	ModelBucket       string     `json:"model_bucket"`
	ModelKey          string     `json:"model_key"`
	Kid               string     `json:"kid"`
	UseDelegated      bool       `json:"use_delegated"`
	UseCoSandbox      bool       `json:"use_co_sandbox"`
	ShmemMB           proc.Tmem  `json:"shmem_mb"`
	UseWriteReadShmem bool       `json:"use_write_read_shmem"`
	Mcpu              proc.Tmcpu `json:"mcpu"`
}

func NewImgrecWASMJobConfig(imgBucket, imgKey, modelBucket, modelKey, kid string, useDelegated, useCoSandbox bool, shmemMB proc.Tmem, useWriteReadShmem bool, mcpu proc.Tmcpu) *ImgrecWASMJobConfig {
	return &ImgrecWASMJobConfig{
		ImgBucket:         imgBucket,
		ImgKey:            imgKey,
		ModelBucket:       modelBucket,
		ModelKey:          modelKey,
		Kid:               kid,
		UseDelegated:      useDelegated,
		UseCoSandbox:      useCoSandbox,
		ShmemMB:           shmemMB,
		UseWriteReadShmem: useWriteReadShmem,
		Mcpu:              mcpu,
	}
}

type ImgrecWASMJob struct {
	conf *ImgrecWASMJobConfig
	*sigmaclnt.SigmaClnt
	coSandbox        []byte
	bootInput        []byte
	precompiledBytes []byte
}

// NewImgrecWASMJob creates a new job, reading and precompiling the WASM binary
// and (if UseCoSandbox) fetching the cosandbox boot script.
func NewImgrecWASMJob(conf *ImgrecWASMJobConfig, sc *sigmaclnt.SigmaClnt) (*ImgrecWASMJob, error) {
	j := &ImgrecWASMJob{conf: conf, SigmaClnt: sc}
	if conf.UseCoSandbox {
		b, err := wasmrt.ReadCoSandbox(sc, cosandboxName)
		if err != nil {
			db.DPrintf(db.ERROR, "ImgrecWASM ReadCoSandbox err: %v", err)
			return nil, err
		}
		j.coSandbox = b
		j.bootInput = wasmrt.EncodeArgs([]string{conf.ImgBucket, conf.ImgKey, conf.ModelBucket, conf.ModelKey, conf.Kid})
	}
	raw, err := readRawWASMBin(sc)
	if err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM read wasm bin err: %v", err)
		return nil, err
	}
	wrt := wasmrt.NewWasmerRuntime(nil)
	compiled, err := wrt.PrecompileModule(raw)
	if err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM PrecompileModule err: %v", err)
		return nil, err
	}
	j.precompiledBytes = compiled
	return j, nil
}

// PrecompileAndUpload reads the raw imgrec WASM binary, precompiles it, and
// uploads it to the remote precompiled-binary path for the given SigmaClnt's
// build tag. Callers that need the binary present before spawning procs (e.g.
// warm-up benchmarks) should call this before spawning.
func PrecompileAndUpload(sc *sigmaclnt.SigmaClnt) error {
	raw, err := readRawWASMBin(sc)
	if err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM PrecompileAndUpload read err: %v", err)
		return err
	}
	wrt := wasmrt.NewWasmerRuntime(nil)
	compiled, err := wrt.PrecompileModule(raw)
	if err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM PrecompileAndUpload precompile err: %v", err)
		return err
	}
	pn := precompiledBinPath(sc)
	sc.Remove(pn)
	fw, err := sc.CreateWriter(pn, 0777, sp.OWRITE)
	if err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM PrecompileAndUpload CreateWriter err: %v", err)
		return err
	}
	if _, err := io.Copy(fw, bytes.NewReader(compiled)); err != nil {
		fw.Close()
		db.DPrintf(db.ERROR, "ImgrecWASM PrecompileAndUpload upload err: %v", err)
		return err
	}
	return fw.Close()
}

// Run uploads the precompiled WASM binary, spawns the proc, waits for
// completion, and returns the result message (class_idx,score).
func (j *ImgrecWASMJob) Run(sigmaPath string) (string, error) {
	precompiledPath := precompiledBinPath(j.SigmaClnt)
	j.Remove(precompiledPath)
	wrt, err := j.CreateWriter(precompiledPath, 0777, sp.OWRITE)
	if err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM CreateWriter err: %v", err)
		return "", err
	}
	if _, err := io.Copy(wrt, bytes.NewReader(j.precompiledBytes)); err != nil {
		wrt.Close()
		db.DPrintf(db.ERROR, "ImgrecWASM upload precompiled wasm err: %v", err)
		return "", err
	}
	wrt.Close()

	p := proc.NewProc(PrecompiledProg, []string{
		j.conf.ImgBucket, j.conf.ImgKey,
		j.conf.ModelBucket, j.conf.ModelKey,
		j.conf.Kid,
	})
	p.GetProcEnv().UseSPProxy = true
	p.SetProcContainerType(proc.ProcContainerType_PROC_CTR_WASM)
	if j.conf.Mcpu > 0 {
		p.SetMcpu(j.conf.Mcpu)
	}
	if j.conf.UseCoSandbox {
		p.SetCoSandbox(j.coSandbox, j.bootInput)
		p.SetRunCoSandbox(true)
	}
	p.SetWasmBufMB(200)
	if j.conf.ShmemMB > 0 {
		p.SetShmemMB(j.conf.ShmemMB)
	} else if j.conf.UseWriteReadShmem {
		// WriteRead shmem requires a shmem segment; allocate a default if none specified.
		p.SetShmemMB(proc.Tmem(32))
	}
	if j.ProcEnv().BuildTag == sp.LOCAL_BUILD {
		p.PrependSigmaPath(filepath.Dir(precompiledPath))
	}
	if sigmaPath != sp.NOT_SET {
		p.PrependSigmaPath(sigmaPath)
	}
	db.DPrintf(db.TEST, "Scale %v", p.GetPid())
	if err := j.Spawn(p); err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM Spawn err: %v", err)
		return "", err
	}
	if err := j.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM WaitStart err: %v", err)
		return "", err
	}
	status, err := j.WaitExit(p.GetPid())
	if err != nil {
		db.DPrintf(db.ERROR, "ImgrecWASM WaitExit err: %v", err)
		return "", err
	}
	if !status.IsStatusOK() {
		return "", fmt.Errorf("imgrec_precompiled.wasm exited with status: %v", status)
	}
	return status.Msg(), nil
}

// readRawWASMBin reads the raw imgrec.wasm binary. For local builds it reads
// from the local filesystem; for remote builds it reads from S3.
func readRawWASMBin(sc *sigmaclnt.SigmaClnt) ([]byte, error) {
	if sc.ProcEnv().BuildTag == sp.LOCAL_BUILD {
		_, b, _, _ := runtime.Caller(0)
		// b is .../apps/imgrec/wasm/job.go; go up 4 levels to project root
		projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(b))))
		return os.ReadFile(filepath.Join(projectRoot, "bin/wasm/imgrec.wasm"))
	}
	pn := filepath.Join(sp.S3, sp.ANY, sc.ProcEnv().BuildTag, "wasm", "imgrec.wasm")
	rdr, err := sc.OpenReader(pn)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	return io.ReadAll(rdr)
}

func precompiledBinPath(sc *sigmaclnt.SigmaClnt) sp.Tsigmapath {
	if sc.ProcEnv().BuildTag == sp.LOCAL_BUILD {
		return filepath.Join(sp.S3, sp.ANY, "9ps3", PrecompiledProg+"-v"+sp.Version)
	}
	return filepath.Join(sp.S3, sp.ANY, sc.ProcEnv().BuildTag, "bin", PrecompiledProg+"-v"+sp.Version)
}
