package mr

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/ft/task"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/coordination/semaphore"
	"sigmaos/util/yaml"

	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_srv "sigmaos/ft/task/srv"
)

const (
	MR          = "/mr/"
	MRDIRTOP    = "name/" + MR
	MRDIRELECT  = "name/mr-elect"
	OUTLINK     = "output"
	INT_OUTLINK = "intermediate-output"
	JOBSEM      = "jobsem"
	SPLITSZ     = 10 * sp.MBYTE
)

func JobOut(outDir, job string) string {
	return filepath.Join(outDir, job)
}

func JobOutLink(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), OUTLINK)
}

func JobIntOutLink(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), INT_OUTLINK)
}

func LeaderElectDir(job string) string {
	return filepath.Join(MRDIRELECT, job)
}

func JobDir(jobRoot, job string) string {
	return filepath.Join(jobRoot, job)
}

func JobSem(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), JOBSEM)
}

func MRstats(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), "stats.txt")
}

func MapTask(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), "/m")
}

func MapIntermediateDir(job, intOutdir string) string {
	return filepath.Join(intOutdir, job)
}

func ReduceTask(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), "/r")
}

func ReduceIn(jobRoot, job string) string {
	return JobDir(jobRoot, job) + "-rin/"
}

func ReduceOut(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), "mr-out-")
}

func ReduceOutTarget(outDir string, job string) string {
	return filepath.Join(JobOut(outDir, job), "mr-out-")
}

func BinName(i int) string {
	return fmt.Sprintf("bin%04d", i)
}

func mshardfile(dir string, r int) string {
	return filepath.Join(dir, "r-"+strconv.Itoa(r)+"-")
}

type Job struct {
	App          string `yalm:"app"`
	Nreduce      int    `yalm:"nreduce"`
	Binsz        int    `yalm:"binsz"`
	Input        string `yalm:"input"`
	Intermediate string `yalm:"intermediate"`
	Output       string `yalm:"output"`
	Linesz       int    `yalm:"linesz"`
	Wordsz       int    `yalm:"wordsz"`
	Local        string `yalm:"input"`
}

// Wait until the job is done
func WaitJobDone(fsl *fslib.FsLib, jobRoot, job string) error {
	sc := semaphore.NewSemaphore(fsl, JobSem(jobRoot, job))
	return sc.Down()
}

func InitJobSem(fsl *fslib.FsLib, jobRoot, job string) error {
	sc := semaphore.NewSemaphore(fsl, JobSem(jobRoot, job))
	return sc.Init(0)
}

func JobDone(fsl *fslib.FsLib, jobRoot, job string) {
	sc := semaphore.NewSemaphore(fsl, JobSem(jobRoot, job))
	sc.Up()
}

func ReadJobConfig(app string) (*Job, error) {
	job := &Job{}
	if err := yaml.ReadYaml(app, job); err != nil {
		db.DPrintf(db.ERROR, "ReadConfig err %v\n", err)
		return nil, err
	}
	return job, nil
}

type Tasks struct {
	Mftsrv  *fttask_srv.FtTaskSrvMgr
	Mftclnt fttask_clnt.FtTaskClnt[Bin, any]

	Rftsrv  *fttask_srv.FtTaskSrvMgr
	Rftclnt fttask_clnt.FtTaskClnt[TreduceTask, any]
}

func InitCoordFS(sc *sigmaclnt.SigmaClnt, jobRoot, jobname string, nreducetask int) (*Tasks, error) {
	sc.FsLib.MkDir(MRDIRTOP, 0777)
	sc.FsLib.MkDir(MRDIRELECT, 0777)
	sc.FsLib.MkDir(jobRoot, 0777)

	mftsrv, err := fttask_srv.NewFtTaskSrvMgr(sc, jobname+"-mtasks", true)
	if err != nil {
		db.DPrintf(db.ERROR, "NewFtTaskSrvMgr %v err %v\n", jobname, err)
		return nil, err
	}
	mftclnt := fttask_clnt.NewFtTaskClnt[Bin, any](sc.FsLib, mftsrv.Id)

	rftsrv, err := fttask_srv.NewFtTaskSrvMgr(sc, jobname+"-rtasks", true)
	if err != nil {
		db.DPrintf(db.ERROR, "NewFtTaskSrvMgr %v err %v\n", jobname, err)
		return nil, err
	}
	rftclnt := fttask_clnt.NewFtTaskClnt[TreduceTask, any](sc.FsLib, rftsrv.Id)

	dirs := []string{
		JobDir(jobRoot, jobname),
		LeaderElectDir(jobname),
		MapTask(jobRoot, jobname),
		ReduceTask(jobRoot, jobname),
	}
	for _, n := range dirs {
		if err := sc.FsLib.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return nil, err
		}
	}
	if err := InitJobSem(sc.FsLib, jobRoot, jobname); err != nil {
		db.DPrintf(db.ERROR, "Err init job sem")
		return nil, err
	}

	// Submit reduce task
	rTasks := make([]*fttask_clnt.Task[TreduceTask], nreducetask)
	for r := 0; r < nreducetask; r++ {
		t := TreduceTask{strconv.Itoa(r), nil}
		rTasks[r] = &fttask_clnt.Task[TreduceTask]{Id: fttask_clnt.TaskId(r), Data: t}
	}
	_, err = rftclnt.SubmitTasks(rTasks)
	return &Tasks{mftsrv, mftclnt, rftsrv, rftclnt}, err
}

// Clean up all old MR outputs
func CleanupMROutputs(fsl *fslib.FsLib, outputDir, intOutputDir string, swapLocalForAny bool) error {
	db.DPrintf(db.MR, "Clean up MR outputs: %v %v", outputDir, intOutputDir)
	defer db.DPrintf(db.MR, "Clean up MR outputs done")

	fsl.RmDir(intOutputDir)
	oDir := outputDir
	if swapLocalForAny {
		oDir = strings.ReplaceAll(oDir, sp.LOCAL, sp.ANY)
	}
	return fsl.RmDir(oDir)
}

func JobLocalToAny(j *Job, input, intermediate, output bool) *Job {
	// Make a copy of the job struct so we can adjust some paths (e.g., replace
	// ~local with ~any), for the test program
	job := &Job{}
	*job = *j
	if input && strings.Contains(job.Input, sp.LOCAL) {
		job.Input = strings.ReplaceAll(job.Input, sp.LOCAL, sp.ANY)
	}
	if intermediate && strings.Contains(job.Intermediate, sp.LOCAL) {
		job.Intermediate = strings.ReplaceAll(job.Intermediate, sp.LOCAL, sp.ANY)
	}
	if output && strings.Contains(job.Output, sp.LOCAL) {
		job.Output = strings.ReplaceAll(job.Output, sp.LOCAL, sp.ANY)
	}
	return job
}

func PrepareJob(fsl *fslib.FsLib, ts *Tasks, jobRoot, jobName string, j *Job) (int, error) {
	job := JobLocalToAny(j, false, false, true)
	db.DPrintf(db.TEST, "job %v", job)

	if job.Output == "" || job.Intermediate == "" {
		return 0, fmt.Errorf("Err job output (\"%v\") or intermediate (\"%v\") not supplied", job.Output, job.Intermediate)
	}
	fsl.MkDir(job.Output, 0777)
	outDir := JobOut(job.Output, jobName)
	if err := fsl.MkDir(outDir, 0777); err != nil {
		db.DPrintf(db.ALWAYS, "Error mkdir job dir %v: %v", outDir, err)
		return 0, err
	}
	if _, err := fsl.PutFile(JobOutLink(jobRoot, jobName), 0777, sp.OWRITE, []byte(job.Output)); err != nil {
		db.DPrintf(db.ALWAYS, "Error link output dir [%v] [%v]: %v", job.Output, JobOutLink(jobRoot, jobName), err)
		return 0, err
	}

	// If intermediate output directory lives in S3, make it only
	// once.  Mappers make intermediate and out dirs in their local ux
	if strings.Contains(job.Intermediate, "/s3/") {
		intOutDir := MapIntermediateDir(jobName, job.Intermediate)
		if err := fsl.MkDir(job.Intermediate, 0777); err != nil {
			return 0, err
		}
		if err := fsl.MkDir(intOutDir, 0777); err != nil {
			return 0, err
		}
	}

	if _, err := fsl.PutFile(JobIntOutLink(jobRoot, jobName), 0777, sp.OWRITE, []byte(job.Intermediate)); err != nil {
		db.DPrintf(db.ALWAYS, "Error link intermediate dir [%v] [%v]: %v", job.Output, JobOutLink(jobRoot, jobName), err)
		return 0, err
	}

	splitsz := sp.Tlength(SPLITSZ)

	bins, err := NewBins(fsl, job.Input, true, sp.Tlength(job.Binsz), splitsz)
	if err != nil || len(bins) == 0 {
		return len(bins), err
	}

	mtasks := make([]*fttask_clnt.Task[Bin], len(bins))
	for i, b := range bins {
		mtasks[i] = &fttask_clnt.Task[Bin]{Id: fttask_clnt.TaskId(i), Data: b}
	}
	_, err = ts.Mftclnt.SubmitTasks(mtasks)
	return len(bins), err
}

func CreateMapperIntOutDirUx(fsl *fslib.FsLib, job, intOutput string) error {
	if strings.Contains(intOutput, "/ux/") {
		if _, err := fsl.Stat(intOutput); err != nil {
			if err := fsl.MkDir(intOutput, 0777); err != nil {
				if !serr.IsErrorExists(err) {
					return err
				}
			}
		}
		intOutDir := MapIntermediateDir(job, intOutput)
		if _, err := fsl.Stat(intOutDir); err != nil {
			if err := fsl.MkDir(intOutDir, 0777); err != nil {
				if serr.IsErrorExists(err) {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

func StartMRJob(sc *sigmaclnt.SigmaClnt, jobRoot, jobName string, job *Job, nmap int, memPerTask proc.Tmem, maliciousMapper int, mftid task.FtTaskSrvId, rftid task.FtTaskSrvId) *procgroupmgr.ProcGroupMgr {
	cfg := procgroupmgr.NewProcGroupConfig(NCOORD, "mr-coord",
		[]string{
			jobRoot,
			strconv.Itoa(nmap),
			strconv.Itoa(job.Nreduce),
			"mr-m-" + job.App,
			"mr-r-" + job.App,
			strconv.Itoa(job.Linesz),
			strconv.Itoa(job.Wordsz),
			strconv.Itoa(int(memPerTask)),
			strconv.Itoa(maliciousMapper),
			string(mftid),
			string(rftid),
		}, 1000, jobName)
	return cfg.StartGrpMgr(sc)
}

// XXX run as a proc?
func MergeReducerOutput(fsl *fslib.FsLib, jobRoot, jobName, out string, nreduce int) error {
	file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		db.DPrintf(db.MR, "Error OpenFile out: %v", err)
		return err
	}
	defer file.Close()
	wrt := bufio.NewWriter(file)
	for i := 0; i < nreduce; i++ {
		r := strconv.Itoa(i)
		rdr, err := fsl.OpenReader(ReduceOut(jobRoot, jobName) + r + "/")
		if err != nil {
			db.DPrintf(db.MR, "Error OpenReader [%v]: %v", ReduceOut(jobRoot, jobName)+r+"/", err)
			return err
		}
		if _, err := io.Copy(wrt, rdr); err != nil {
			db.DPrintf(db.MR, "Error Copy: %v", err)
			return err
		}
	}
	return nil
}
