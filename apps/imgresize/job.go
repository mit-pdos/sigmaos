package imgresize

import (
	"path/filepath"
	"strconv"
	"strings"

	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_coord "sigmaos/ft/task/coord"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
	rd "sigmaos/util/rand"
)

type Ttask struct {
	FileName string `json:"File"`
}

func NewTask(fn string) *Ttask {
	return &Ttask{fn}
}

func StartImgd(sc *sigmaclnt.SigmaClnt, srvId task.FtTaskSrvId, workerMcpu proc.Tmcpu, workerMem proc.Tmem, persist bool, nrounds int, imgdMcpu proc.Tmcpu, em *crash.TeventMap) *procgroupmgr.ProcGroupMgr {
	crash.SetSigmaFail(em)
	cfg := procgroupmgr.NewProcGroupConfig(1, "imgresized", []string{strconv.Itoa(int(workerMcpu)), strconv.Itoa(int(workerMem)), strconv.Itoa(nrounds)}, imgdMcpu, string(srvId))
	if persist {
		cfg.Persist(sc.FsLib)
	}
	return cfg.StartGrpMgr(sc)
}

func StartImgRPCd(sc *sigmaclnt.SigmaClnt, job string, workerMcpu proc.Tmcpu, workerMem proc.Tmem, nrounds int, imgdMcpu proc.Tmcpu) (*proc.Proc, error) {
	p := proc.NewProc("imgresizerpcd", []string{job, strconv.Itoa(int(workerMcpu)), strconv.Itoa(int(workerMem)), strconv.Itoa(nrounds)})
	p.SetMcpu(imgdMcpu)
	if err := sc.Spawn(p); err != nil {
		db.DPrintf(db.TEST, "Error Spawn %v", p)
		return p, err
	}
	if err := sc.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.TEST, "Error WaitStart %v", p)
		return p, err
	}
	return p, nil
}

// remove old thumbnails
func Cleanup(fsl *fslib.FsLib, dir string) error {
	_, err := fsl.ProcessDir(dir, func(st *sp.Tstat) (bool, error) {
		if IsThumbNail(st.Name) {
			err := fsl.Remove(filepath.Join(dir, st.Name))
			if err != nil {
				return true, err
			}
			return false, nil
		}
		return false, nil
	})
	return err
}

func ThumbName(fn string) string {
	ext := filepath.Ext(fn)
	fn1 := strings.TrimSuffix(fn, ext) + "-" + rd.String(4) + "-thumb" + filepath.Ext(fn)
	return fn1
}

func IsThumbNail(fn string) bool {
	return strings.Contains(fn, "-thumb")
}

func getMkProcFn(serverId task.FtTaskSrvId, nrounds int, workerMcpu proc.Tmcpu, workerMem proc.Tmem) fttask_coord.TmkProc[Ttask] {
	return func(task fttask_clnt.Task[Ttask]) *proc.Proc {
		db.DPrintf(db.IMGD, "mkProc %v", task)
		fn := task.Data.FileName
		p := proc.NewProcPid(sp.GenPid(string(serverId)), "imgresize", []string{fn, ThumbName(fn), strconv.Itoa(nrounds)})
		p.SetMcpu(workerMcpu)
		p.SetMem(workerMem)
		return p
	}
}
