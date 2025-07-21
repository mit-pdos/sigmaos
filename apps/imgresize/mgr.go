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
	fttask_srv "sigmaos/ft/task/srv"
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

type ImgdMgr[Data any] struct {
	job   string
	pgm   *procgroupmgr.ProcGroupMgr
	ftsrv *fttask_srv.FtTaskSrvMgr
}

func NewImgdMgr[Data any](sc *sigmaclnt.SigmaClnt, job string, workerMcpu proc.Tmcpu, workerMem proc.Tmem, persist bool, nrounds int, imgdMcpu proc.Tmcpu, em *crash.TeventMap) (*ImgdMgr[Data], error) {
	crash.SetSigmaFail(em)
	imgd := &ImgdMgr[Data]{}

	imgd.job = job
	var err error
	imgd.ftsrv, err = fttask_srv.NewFtTaskSrvMgr(sc, TaskSvcId(job), false)
	if err != nil {
		return nil, err
	}

	cfg := procgroupmgr.NewProcGroupConfig(1, "imgresized", []string{strconv.Itoa(int(workerMcpu)), strconv.Itoa(int(workerMem)), strconv.Itoa(nrounds), TaskSvcId(imgd.job)}, imgdMcpu, ImgSvcId(job))

	if persist {
		cfg.Persist(sc.FsLib)
	}
	imgd.pgm = cfg.StartGrpMgr(sc)
	return imgd, nil
}

func (imgd *ImgdMgr[Data]) NewImgdClnt(sc *sigmaclnt.SigmaClnt) (*ImgdClnt[Data], error) {
	return NewImgdClnt[Data](sc, imgd.job, imgd.ftsrv.Id)
}

func (imgd *ImgdMgr[Data]) Restart(sc *sigmaclnt.SigmaClnt) error {
	var err error
	imgd.ftsrv, err = fttask_srv.NewFtTaskSrvMgr(sc, TaskSvcId(imgd.job), false)
	if err != nil {
		return err
	}
	pgms, err := procgroupmgr.Recover(sc)
	if err != nil {
		return err
	}
	if len(pgms) < 1 {
		fmt.Errorf("Too few procgroup mgrs")
	}
	imgd.pgm = pgms[0]
	return nil
}

func (imgd *ImgdMgr[Data]) WaitImgd() []*procgroupmgr.ProcStatus {
	sts := imgd.pgm.WaitGroup()
	imgd.ftsrv.Stop(true)
	return sts
}

func (imgd *ImgdMgr[Data]) StopImgd(clearStore bool) ([]*procgroupmgr.ProcStatus, error) {
	sts, err := imgd.pgm.StopGroup()
	imgd.ftsrv.Stop(clearStore)
	return sts, err
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
