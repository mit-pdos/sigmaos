package imgresize

import (
	"path/filepath"
	"strconv"
	"strings"

	"sigmaos/util/crash"
	db "sigmaos/debug"
	"sigmaos/fslib"
	fttaskmgr"sigmaos/ft/task/mgr"
	"sigmaos/ft/groupmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	rd "sigmaos/util/rand"
)

type Ttask struct {
	FileName string `json:"File"`
}

func NewTask(fn string) *Ttask {
	return &Ttask{fn}
}

func StartImgd(sc *sigmaclnt.SigmaClnt, job string, workerMcpu proc.Tmcpu, workerMem proc.Tmem, persist bool, nrounds int, imgdMcpu proc.Tmcpu, evs []crash.Tevent) *groupmgr.GroupMgr {
	crash.SetSigmaFail(evs)
	cfg := groupmgr.NewGroupConfig(1, "imgresized", []string{strconv.Itoa(int(workerMcpu)), strconv.Itoa(int(workerMem)), strconv.Itoa(nrounds)}, imgdMcpu, job)
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

func getMkProcFn(job string, nrounds int, workerMcpu proc.Tmcpu, workerMem proc.Tmem) fttaskmgr.TmkProc {
	return func(tn string, t interface{}) *proc.Proc {
		task := *t.(*Ttask)
		db.DPrintf(db.IMGD, "mkProc %s %v", tn, task)
		fn := task.FileName
		p := proc.NewProcPid(sp.GenPid(job), "imgresize", []string{fn, ThumbName(fn), strconv.Itoa(nrounds)})
		p.SetMcpu(workerMcpu)
		p.SetMem(workerMem)
		return p
	}
}
