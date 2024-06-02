package imgresizesrv

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/fttaskmgr"
	"sigmaos/fttasks"
	"sigmaos/leaderclnt"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	IMG = "name/img"
)

type Ttask struct {
	FileName string `json:"File"`
}

func NewTask(fn string) *Ttask {
	return &Ttask{fn}
}

type ImgSrv struct {
	*sigmaclnt.SigmaClnt
	ft         *fttasks.FtTasks
	job        string
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	crash      int64
	exited     bool
	leaderclnt *leaderclnt.LeaderClnt
	stop       int32
}

// remove old thumbnails
func Cleanup(fsl *fslib.FsLib, dir string) error {
	_, err := fsl.ProcessDir(dir, func(st *sp.Stat) (bool, error) {
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

func NewImgSrv(args []string) (*ImgSrv, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("NewImgSrv: wrong number of arguments: %v", args)
	}
	imgd := &ImgSrv{}
	imgd.job = args[0]
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.IMGD, "Made fslib job %v", imgd.job)
	imgd.SigmaClnt = sc
	imgd.job = args[0]
	crashing, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: error parse crash %v", err)
	}
	imgd.crash = int64(crashing)
	imgd.ft, err = fttasks.NewFtTasks(sc.FsLib, IMG, imgd.job)
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: NewFtTasks %v", err)
	}
	mcpu, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse MCPU %v", err)
	}
	imgd.workerMcpu = proc.Tmcpu(mcpu)
	mem, err := strconv.Atoi(args[3])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse Mem %v", err)
	}
	imgd.workerMem = proc.Tmem(mem)
	imgd.nrounds, err = strconv.Atoi(args[4])
	if err != nil {
		db.DFatalf("Error parse nrounds: %v", err)
	}

	imgd.Started()

	imgd.leaderclnt, err = leaderclnt.NewLeaderClnt(imgd.FsLib, filepath.Join(IMG, imgd.job, "imgd-leader"), 0777)
	if err != nil {
		return nil, fmt.Errorf("NewLeaderclnt err %v", err)
	}

	crash.Crasher(imgd.FsLib)

	go func() {
		imgd.WaitEvict(sc.ProcEnv().GetPID())
		if !imgd.exited {
			imgd.ClntExitOK()
		}
		os.Exit(0)
	}()

	return imgd, nil
}

func ThumbName(fn string) string {
	ext := filepath.Ext(fn)
	fn1 := strings.TrimSuffix(fn, ext) + "-" + rd.String(4) + "-thumb" + filepath.Ext(fn)
	return fn1
}

func IsThumbNail(fn string) bool {
	return strings.Contains(fn, "-thumb")
}

func (imgd *ImgSrv) mkProc(tn string, t interface{}) *proc.Proc {
	task := *t.(*Ttask)
	db.DPrintf(db.FTTASKS, "mkProc %s %v", tn, task)
	fn := task.FileName
	p := proc.NewProcPid(sp.GenPid(imgd.job), "imgresize", []string{fn, ThumbName(fn), strconv.Itoa(imgd.nrounds)})
	if imgd.crash > 0 {
		p.SetCrash(imgd.crash)
	}
	p.SetMcpu(imgd.workerMcpu)
	p.SetMem(imgd.workerMem)
	return p
}

func (imgd *ImgSrv) Work() {
	db.DPrintf(db.IMGD, "Try acquire leadership coord %v job %v", imgd.ProcEnv().GetPID(), imgd.job)

	// Try to become the leading coordinator.
	if err := imgd.leaderclnt.LeadAndFence(nil, []string{filepath.Join(IMG, imgd.job)}); err != nil {
		sts, err2 := imgd.ft.Jobs()
		db.DFatalf("LeadAndFence err %v sts %v err2 %v", err, sp.Names(sts), err2)
	}

	db.DPrintf(db.ALWAYS, "leader %s", imgd.job)

	ftm, err := fttaskmgr.NewTaskMgr(imgd.SigmaClnt.ProcAPI, imgd.ft)
	if err != nil {
		db.DFatalf("NewTaskMgr err %v", err)
	}
	status := ftm.ExecuteTasks(func() interface{} { return new(Ttask) }, imgd.mkProc)
	db.DPrintf(db.ALWAYS, "imgresized exit")
	imgd.exited = true
	if status == nil {
		imgd.ClntExitOK()
	} else {
		imgd.ClntExit(proc.NewStatusInfo(proc.StatusFatal, "task error", status))
	}
}
