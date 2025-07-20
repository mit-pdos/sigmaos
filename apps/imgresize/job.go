package imgresize

import (
	"fmt"
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

const (
	TASKSVC = "imgresize-tasksvc"
	IMGSVC  = "imgresize"
)

func ImgSvcId(job string) string {
	return fmt.Sprintf("%s-%s", IMGSVC, job)
}

func TaskSvcId(job string) string {
	return fmt.Sprintf("%s-%s", TASKSVC, job)
}

type Ttask struct {
	FileName string `json:"File"`
}

func NewTask(fn string) *Ttask {
	return &Ttask{fn}
}

func StartImgd(sc *sigmaclnt.SigmaClnt, svcId, svcTaskId string, workerMcpu proc.Tmcpu, workerMem proc.Tmem, persist bool, nrounds int, imgdMcpu proc.Tmcpu, em *crash.TeventMap) *procgroupmgr.ProcGroupMgr {
	crash.SetSigmaFail(em)
	cfg := procgroupmgr.NewProcGroupConfig(1, "imgresized", []string{strconv.Itoa(int(workerMcpu)), strconv.Itoa(int(workerMem)), strconv.Itoa(nrounds), svcTaskId}, imgdMcpu, svcId)
	if persist {
		cfg.Persist(sc.FsLib)
	}
	return cfg.StartGrpMgr(sc)
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

func GetMkProcFn(serverId task.FtTaskSvcId, nrounds int, workerMcpu proc.Tmcpu, workerMem proc.Tmem) fttask_coord.TmkProc[Ttask] {
	return func(task fttask_clnt.Task[Ttask]) *proc.Proc {
		db.DPrintf(db.IMGD, "mkProc %v", task)
		fn := task.Data.FileName
		p := proc.NewProcPid(sp.GenPid(string(serverId)), "imgresize", []string{fn, ThumbName(fn), strconv.Itoa(nrounds)})
		p.SetMcpu(workerMcpu)
		p.SetMem(workerMem)
		return p
	}
}
