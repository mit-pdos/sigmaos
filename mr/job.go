package mr

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/yaml"
)

const (
	MR          = "/mr/"
	MRDIRTOP    = "name/" + MR
	MRDIRELECT  = "name/mr-elect"
	OUTLINK     = "output"
	INT_OUTLINK = "intermediate-output"
	JOBSEM      = "jobsem"
)

func JobOut(outDir, job string) string {
	return path.Join(outDir, job)
}

func JobOutLink(job string) string {
	return path.Join(JobDir(job), OUTLINK)
}

func JobIntOutLink(job string) string {
	return path.Join(JobDir(job), INT_OUTLINK)
}

func LeaderElectDir(job string) string {
	return path.Join(MRDIRELECT, job)
}

func JobDir(job string) string {
	return path.Join(MRDIRTOP, job)
}

func JobSem(job string) string {
	return path.Join(MRDIRTOP, job, JOBSEM)
}

func MRstats(job string) string {
	return path.Join(JobDir(job), "stats.txt")
}

func MapTask(job string) string {
	return path.Join(JobDir(job), "/m")
}

func MapIntermediateOutDir(job, intOutdir, mapname string) string {
	return path.Join(intOutdir, job, "m-"+mapname)
}

func ReduceTask(job string) string {
	return path.Join(JobDir(job), "/r")
}

func ReduceIn(job string) string {
	return JobDir(job) + "-rin/"
}

func ReduceOut(job string) string {
	return path.Join(JobDir(job), "mr-out-")
}

func ReduceOutTarget(outDir string, job string) string {
	return path.Join(JobOut(outDir, job), "mr-out-")
}

func BinName(i int) string {
	return fmt.Sprintf("bin%04d", i)
}

func mshardfile(dir string, r int) string {
	return path.Join(dir, "r-"+strconv.Itoa(r))
}

func symname(job, r, name string) string {
	return path.Join(ReduceIn(job), r, "m-"+name)
}

type Job struct {
	App          string `yalm:"app"`
	Nreduce      int    `yalm:"nreduce"`
	Binsz        int    `yalm:"binsz"`
	Input        string `yalm:"input"`
	Intermediate string `yalm:"intermediate"`
	Output       string `yalm:"output"`
	Linesz       int    `yalm:"linesz"`
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

func InitCoordFS(fsl *fslib.FsLib, jobname string, nreducetask int) error {
	fsl.MkDir(MRDIRTOP, 0777)
	fsl.MkDir(MRDIRELECT, 0777)
	dirs := []string{
		LeaderElectDir(jobname),
		JobDir(jobname),
		MapTask(jobname),
		ReduceTask(jobname),
		ReduceIn(jobname),
		MapTask(jobname) + TIP,
		ReduceTask(jobname) + TIP,
		MapTask(jobname) + DONE,
		ReduceTask(jobname) + DONE,
		MapTask(jobname) + NEXT,
		ReduceTask(jobname) + NEXT,
	}
	for _, n := range dirs {
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return err
		}
	}

	if err := InitJobSem(fsl, jobname); err != nil {
		db.DPrintf(db.ERROR, "Err init job sem")
		return err
	}

	// Make task and input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := ReduceTask(jobname) + "/" + strconv.Itoa(r)
		if _, err := fsl.PutFile(n, 0777, sp.OWRITE, []byte{}); err != nil {
			db.DPrintf(db.ERROR, "Putfile %v err %v\n", n, err)
			return err
		}
		n = ReduceIn(jobname) + "/" + strconv.Itoa(r)
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return err
		}
	}

	// Create empty stats file
	if _, err := fsl.PutFile(MRstats(jobname), 0777, sp.OWRITE, []byte{}); err != nil {
		db.DPrintf(db.ERROR, "Putfile %v err %v\n", MRstats(jobname), err)
		return err
	}
	return nil
}

// Clean up all old MR outputs
func CleanupMROutputs(fsl *fslib.FsLib, outputDir, intOutputDir string) {
	db.DPrintf(db.MR, "Clean up MR outputs: %v", outputDir)
	fsl.RmDir(outputDir)
	fsl.RmDir(intOutputDir)
	db.DPrintf(db.MR, "Clean up MR outputs done: %v", outputDir)
}

// Put names of input files in name/mr/m
func PrepareJob(fsl *fslib.FsLib, jobName string, job *Job) (int, error) {
	if job.Output == "" || job.Intermediate == "" {
		return 0, fmt.Errorf("Err job output (\"%v\") or intermediate (\"%v\") not supplied", job.Output, job.Intermediate)
	}
	// Only make out dir if it lives in s3
	if strings.Contains(job.Output, "/s3/") {
		fsl.MkDir(job.Output, 0777)
		outDir := JobOut(job.Output, jobName)
		if err := fsl.MkDir(outDir, 0777); err != nil {
			db.DPrintf(db.ALWAYS, "Error mkdir job dir %v: %v", outDir, err)
			return 0, err
		}
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
	for i, b := range bins {
		n := MapTask(jobName) + "/" + BinName(i)
		if _, err := fsl.PutFile(n, 0777, sp.OWRITE, []byte{}); err != nil {
			return len(bins), err
		}
		for _, s := range b {
			if err := fsl.AppendFileJson(n, s); err != nil {
				return len(bins), err
			}
		}
	}
	return len(bins), nil
}

func StartMRJob(sc *sigmaclnt.SigmaClnt, jobname string, job *Job, ncoord, nmap, crashtask, crashcoord int, memPerTask proc.Tmem, asyncrw bool) *groupmgr.GroupMgr {
	cfg := groupmgr.NewGroupConfig(ncoord, "mr-coord", []string{strconv.Itoa(nmap), strconv.Itoa(job.Nreduce), "mr-m-" + job.App, "mr-r-" + job.App, strconv.Itoa(crashtask), strconv.Itoa(job.Linesz), strconv.Itoa(int(memPerTask)), strconv.FormatBool(asyncrw)}, 1000, jobname)
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
		if _, err := io.Copy(wrt, rdr.Reader); err != nil {
			db.DPrintf(db.MR, "Error Copy: %v", err)
			return err
		}
	}
	return nil
}
