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
	"sigmaos/fslib"
	"sigmaos/fttasks"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/yaml"
)

const (
	MR       = "/mr/"
	MRDIRTOP = "name/" + MR
	//MRDIRTOP    = "name/ux/~local/" + MR
	MRDIRELECT  = "name/mr-elect"
	OUTLINK     = "output"
	INT_OUTLINK = "intermediate-output"
	JOBSEM      = "jobsem"
)

func JobOut(outDir, job string) string {
	return filepath.Join(outDir, job)
}

func JobOutLink(job string) string {
	return filepath.Join(JobDir(job), OUTLINK)
}

func JobIntOutLink(job string) string {
	return filepath.Join(JobDir(job), INT_OUTLINK)
}

func LeaderElectDir(job string) string {
	return filepath.Join(MRDIRELECT, job)
}

func JobDir(job string) string {
	return filepath.Join(MRDIRTOP, job)
}

func JobSem(job string) string {
	return filepath.Join(MRDIRTOP, job, JOBSEM)
}

func MRstats(job string) string {
	return filepath.Join(JobDir(job), "stats.txt")
}

func MapTask(job string) string {
	return filepath.Join(JobDir(job), "/m")
}

func MapIntermediateOutDir(job, intOutdir, mapname string) string {
	return filepath.Join(intOutdir, job, "m-"+mapname)
}

func MapIntermediateDir(job, intOutdir string) string {
	return filepath.Join(intOutdir, job)
}

func ReduceTask(job string) string {
	return filepath.Join(JobDir(job), "/r")
}

func ReduceIn(job string) string {
	return JobDir(job) + "-rin/"
}

func ReduceOut(job string) string {
	return filepath.Join(JobDir(job), "mr-out-")
}

func ReduceOutTarget(outDir string, job string) string {
	return filepath.Join(JobOut(outDir, job), "mr-out-")
}

func BinName(i int) string {
	return fmt.Sprintf("bin%04d", i)
}

func mshardfile(dir string, r int) string {
	return filepath.Join(dir, "r-"+strconv.Itoa(r))
}

func symname(job, r, name string) string {
	return filepath.Join(ReduceIn(job), r, "m-"+name)
}

type Job struct {
	App          string `yalm:"app"`
	Nreduce      int    `yalm:"nreduce"`
	Binsz        int    `yalm:"binsz"`
	Input        string `yalm:"input"`
	Intermediate string `yalm:"intermediate"`
	Output       string `yalm:"output"`
	Linesz       int    `yalm:"linesz"`
	Local        string `yalm:"input"`
}

// Wait until the job is done
func WaitJobDone(fsl *fslib.FsLib, job string) error {
	sc := semclnt.NewSemClnt(fsl, JobSem(job))
	return sc.Down()
}

func InitJobSem(fsl *fslib.FsLib, job string) error {
	sc := semclnt.NewSemClnt(fsl, JobSem(job))
	return sc.Init(0)
}

func JobDone(fsl *fslib.FsLib, job string) {
	sc := semclnt.NewSemClnt(fsl, JobSem(job))
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
	Mft *fttasks.FtTasks
	Rft *fttasks.FtTasks
}

func InitCoordFS(fsl *fslib.FsLib, jobname string, nreducetask int) (*Tasks, error) {
	fsl.MkDir(MRDIRTOP, 0777)
	fsl.MkDir(MRDIRELECT, 0777)

	mft, err := fttasks.MkFtTasks(fsl, MRDIRTOP, filepath.Join(jobname, "/mtasks"))
	if err != nil {
		db.DPrintf(db.ERROR, "MkFtTasks %v err %v\n", jobname, err)
		return nil, err
	}
	rft, err := fttasks.MkFtTasks(fsl, MRDIRTOP, filepath.Join(jobname, "/rtasks"))
	if err != nil {
		db.DPrintf(db.ERROR, "MkFtTasks %v err %v\n", jobname, err)
		return nil, err
	}

	dirs := []string{
		LeaderElectDir(jobname),
		MapTask(jobname),
		ReduceTask(jobname),
		ReduceIn(jobname),
	}
	for _, n := range dirs {
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return nil, err
		}
	}
	if err := InitJobSem(fsl, jobname); err != nil {
		db.DPrintf(db.ERROR, "Err init job sem")
		return nil, err
	}

	// Make input directories for reduce tasks and submit task
	for r := 0; r < nreducetask; r++ {
		rs := strconv.Itoa(r)
		n := ReduceIn(jobname) + "/" + rs
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return nil, err
		}
		t := &TreduceTask{rs}
		if err := rft.SubmitTask(0, t); err != nil {
			db.DPrintf(db.ERROR, "SubmitTask %v err %v\n", t, err)
			return nil, err
		}
	}

	// Create empty stats file
	if _, err := fsl.PutFile(MRstats(jobname), 0777, sp.OWRITE, []byte{}); err != nil {
		db.DPrintf(db.ERROR, "Putfile %v err %v\n", MRstats(jobname), err)
		return nil, err
	}
	return &Tasks{mft, rft}, nil
}

// Clean up all old MR outputs
func CleanupMROutputs(fsl *fslib.FsLib, outputDir, intOutputDir string) {
	db.DPrintf(db.MR, "Clean up MR outputs: %v %v", outputDir, intOutputDir)
	fsl.RmDir(outputDir)
	fsl.RmDir(intOutputDir)
	db.DPrintf(db.MR, "Clean up MR outputs done")
}

// Put names of input files in name/mr/m
func PrepareJob(fsl *fslib.FsLib, ts *Tasks, jobName string, job *Job) (int, error) {
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
	if _, err := fsl.PutFile(JobOutLink(jobName), 0777, sp.OWRITE, []byte(job.Output)); err != nil {
		db.DPrintf(db.ALWAYS, "Error link output dir [%v] [%v]: %v", job.Output, JobOutLink(jobName), err)
		return 0, err
	}
	// Only make intermediate out dir if it lives in s3 (otherwise, it will be
	// made by the mappers on their local machines).
	if strings.Contains(job.Intermediate, "/s3/") {
		fsl.MkDir(job.Intermediate, 0777)
	}
	if _, err := fsl.PutFile(JobIntOutLink(jobName), 0777, sp.OWRITE, []byte(job.Intermediate)); err != nil {
		db.DPrintf(db.ALWAYS, "Error link intermediate dir [%v] [%v]: %v", job.Output, JobOutLink(jobName), err)
		return 0, err
	}

	splitsz := sp.Tlength(10 * sp.MBYTE)
	// splitsz := maxbinsz >> 3 //sp.Tlength(10 * 1024 * 1024)

	bins, err := NewBins(fsl, job.Input, sp.Tlength(job.Binsz), splitsz)
	if err != nil || len(bins) == 0 {
		return len(bins), err
	}
	for _, b := range bins {
		if err := ts.Mft.SubmitTask(0, b); err != nil {
			return len(bins), err
		}

	}
	return len(bins), nil
}

func StartMRJob(sc *sigmaclnt.SigmaClnt, jobname string, job *Job, ncoord, nmap, crashtask, crashcoord int, memPerTask proc.Tmem, asyncrw bool, maliciousMapper int) *groupmgr.GroupMgr {
	cfg := groupmgr.NewGroupConfig(ncoord, "mr-coord", []string{strconv.Itoa(nmap), strconv.Itoa(job.Nreduce), "mr-m-" + job.App, "mr-r-" + job.App, strconv.Itoa(crashtask), strconv.Itoa(job.Linesz), strconv.Itoa(int(memPerTask)), strconv.FormatBool(asyncrw), strconv.Itoa(maliciousMapper)}, 1000, jobname)
	cfg.SetTest(crashcoord, 0, 0)
	return cfg.StartGrpMgr(sc, ncoord)
}

// XXX run as a proc?
func MergeReducerOutput(fsl *fslib.FsLib, jobName, out string, nreduce int) error {
	file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		db.DPrintf(db.MR, "Error OpenFile out: %v", err)
		return err
	}
	defer file.Close()
	wrt := bufio.NewWriter(file)
	for i := 0; i < nreduce; i++ {
		r := strconv.Itoa(i)
		rdr, err := fsl.OpenReader(ReduceOut(jobName) + r + "/")
		if err != nil {
			db.DPrintf(db.MR, "Error OpenReader [%v]: %v", ReduceOut(jobName)+r+"/", err)
			return err
		}
		if _, err := io.Copy(wrt, rdr.(*fslib.FdReader).Reader); err != nil {
			db.DPrintf(db.MR, "Error Copy: %v", err)
			return err
		}
	}
	return nil
}
