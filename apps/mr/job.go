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
	//MRDIRTOP    = "name/ux/~sp.LOCAL/" + MR
	MRDIRELECT  = "name/mr-elect"
	OUTLINK     = "output"
	INT_OUTLINK = "intermediate-output"
	JOBSEM      = "jobsem"
	SPLITSZ     = 10 * sp.MBYTE
	// SPLITSZ = 200 * sp.KBYTE
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
	return filepath.Join(dir, "r-"+strconv.Itoa(r))
}

func symname(jobRoot, job, r, name string) string {
	return filepath.Join(ReduceIn(jobRoot, job), r, "m-"+name)
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
	sc := semclnt.NewSemClnt(fsl, JobSem(jobRoot, job))
	return sc.Down()
}

func InitJobSem(fsl *fslib.FsLib, jobRoot, job string) error {
	sc := semclnt.NewSemClnt(fsl, JobSem(jobRoot, job))
	return sc.Init(0)
}

func JobDone(fsl *fslib.FsLib, jobRoot, job string) {
	sc := semclnt.NewSemClnt(fsl, JobSem(jobRoot, job))
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

func InitCoordFS(fsl *fslib.FsLib, jobRoot, jobname string, nreducetask int) (*Tasks, error) {
	fsl.MkDir(MRDIRTOP, 0777)
	fsl.MkDir(MRDIRELECT, 0777)

	mft, err := fttasks.MkFtTasks(fsl, jobRoot, filepath.Join(jobname, "/mtasks"))
	if err != nil {
		db.DPrintf(db.ERROR, "MkFtTasks %v err %v\n", jobname, err)
		return nil, err
	}
	rft, err := fttasks.MkFtTasks(fsl, jobRoot, filepath.Join(jobname, "/rtasks"))
	if err != nil {
		db.DPrintf(db.ERROR, "MkFtTasks %v err %v\n", jobname, err)
		return nil, err
	}

	dirs := []string{
		LeaderElectDir(jobname),
		MapTask(jobRoot, jobname),
		ReduceTask(jobRoot, jobname),
		ReduceIn(jobRoot, jobname),
	}
	for _, n := range dirs {
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return nil, err
		}
	}
	if err := InitJobSem(fsl, jobRoot, jobname); err != nil {
		db.DPrintf(db.ERROR, "Err init job sem")
		return nil, err
	}

	// Make input directories for reduce tasks and submit task
	for r := 0; r < nreducetask; r++ {
		rs := strconv.Itoa(r)
		n := ReduceIn(jobRoot, jobname) + "/" + rs
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return nil, err
		}
		t := &TreduceTask{rs}
		if err := rft.SubmitTask(r, t); err != nil {
			db.DPrintf(db.ERROR, "SubmitTask %v err %v\n", t, err)
			return nil, err
		}
	}

	// Create empty stats file
	if _, err := fsl.PutFile(MRstats(jobRoot, jobname), 0777, sp.OWRITE, []byte{}); err != nil {
		db.DPrintf(db.ERROR, "Putfile %v err %v\n", MRstats(jobRoot, jobname), err)
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
func PrepareJob(fsl *fslib.FsLib, ts *Tasks, jobRoot, jobName string, job *Job) (int, error) {
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
	redOutDir := ReduceOutTarget(job.Output, jobName)
	intOutDir := MapIntermediateDir(jobName, job.Intermediate)
	// If intermediate output directory lives in S3, make it only once.
	// Otherwise, make it on every node
	if strings.Contains(job.Intermediate, "/s3/") {
		if err := fsl.MkDir(job.Intermediate, 0777); err != nil {
			return 0, err
		}
		if err := fsl.MkDir(intOutDir, 0777); err != nil {
			return 0, err
		}
		if err := fsl.MkDir(redOutDir, 0777); err != nil {
			return 0, err
		}
	} else if strings.Contains(job.Intermediate, "/ux/") {
		uxSts, err := fsl.GetDir(sp.UX)
		if err != nil {
			return 0, err
		}
		for _, ux := range sp.Names(uxSts) {
			intResolved := strings.ReplaceAll(job.Intermediate, sp.LOCAL, ux)
			if err := fsl.MkDir(intResolved, 0777); err != nil {
				return 0, err
			}
			intOutResolved := strings.ReplaceAll(intOutDir, sp.LOCAL, ux)
			if err := fsl.MkDir(intOutResolved, 0777); err != nil {
				return 0, err
			}
			redOutResolved := strings.ReplaceAll(redOutDir, sp.LOCAL, ux)
			if err := fsl.MkDir(redOutResolved, 0777); err != nil {
				return 0, err
			}
		}
	} else {
		return 0, fmt.Errorf("Unknown intermediate job location")
	}
	if _, err := fsl.PutFile(JobIntOutLink(jobRoot, jobName), 0777, sp.OWRITE, []byte(job.Intermediate)); err != nil {
		db.DPrintf(db.ALWAYS, "Error link intermediate dir [%v] [%v]: %v", job.Output, JobOutLink(jobRoot, jobName), err)
		return 0, err
	}

	splitsz := sp.Tlength(SPLITSZ)

	bins, err := NewBins(fsl, job.Input, sp.Tlength(job.Binsz), splitsz)
	if err != nil || len(bins) == 0 {
		return len(bins), err
	}
	for i, b := range bins {
		if err := ts.Mft.SubmitTask(i, b); err != nil {
			return len(bins), err
		}

	}
	return len(bins), nil
}

func StartMRJob(sc *sigmaclnt.SigmaClnt, jobRoot, jobName string, job *Job, nmap int, memPerTask proc.Tmem, maliciousMapper int) *groupmgr.GroupMgr {
	cfg := groupmgr.NewGroupConfig(NCOORD, "mr-coord", []string{jobRoot, strconv.Itoa(nmap), strconv.Itoa(job.Nreduce), "mr-m-" + job.App, "mr-r-" + job.App, strconv.Itoa(job.Linesz), strconv.Itoa(job.Wordsz), strconv.Itoa(int(memPerTask)), strconv.Itoa(maliciousMapper)}, 1000, jobName)
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