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

func JobOut(outDir, job string) string {
	return path.Join(outDir, job)
}

func JobOutLink(job string) string {
	return path.Join(JobDir(job), OUTLINK)
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

func LocalOut(job string) string {
	return path.Join(MLOCALDIR, job)
}

func Moutdir(job, name string) string {
	return path.Join(LocalOut(job), "m-"+name)
}

func mshardfile(job, name string, r int) string {
	return path.Join(Moutdir(job, name), "r-"+strconv.Itoa(r))
}

func shardtarget(job, pn, name string, r int) string {
	return path.Join(pn, MR, job, "m-"+name, "r-"+strconv.Itoa(r)) + "/"
}

func symname(job, r, name string) string {
	return path.Join(ReduceIn(job), r, "m-"+name)
}

type Job struct {
	App     string `yalm:"app"`
	Nreduce int    `yalm:"nreduce"`
	Binsz   int    `yalm:"binsz"`
	Input   string `yalm:"input"`
	Output  string `yalm:"output"`
	Linesz  int    `yalm:"linesz"`
}

// Wait until the job is done
func WaitJobDone(fsl *fslib.FsLib, job string) error {
	sc := semclnt.NewSemClnt(fsl, JobSem(job))
	return sc.Down()
}

func JobDone(fsl *fslib.FsLib, job string) {
	sc := semclnt.NewSemClnt(fsl, JobSem(job))
	sc.Up()
}

func ReadJobConfig(app string) *Job {
	job := &Job{}
	if err := yaml.ReadYaml(app, job); err != nil {
		db.DFatalf("ReadConfig err %v\n", err)
	}
	return job
}

func InitCoordFS(fsl *fslib.FsLib, jobname string, nreducetask int) {
	fsl.MkDir(MRDIRTOP, 0777)
	dirs := []string{
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
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Make task and input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := ReduceTask(jobname) + "/" + strconv.Itoa(r)
		if _, err := fsl.PutFile(n, 0777, sp.OWRITE, []byte{}); err != nil {
			db.DFatalf("Putfile %v err %v\n", n, err)
		}
		n = ReduceIn(jobname) + "/" + strconv.Itoa(r)
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Create empty stats file
	if _, err := fsl.PutFile(MRstats(jobname), 0777, sp.OWRITE, []byte{}); err != nil {
		db.DFatalf("Putfile %v err %v\n", MRstats(jobname), err)
	}
}

// Clean up all old MR outputs
func CleanupMROutputs(fsl *fslib.FsLib, outputDir string) {
	db.DPrintf(db.MR, "Clean up MR outputs: %v", outputDir)
	fsl.RmDir(outputDir)
}

// Put names of input files in name/mr/m
func PrepareJob(fsl *fslib.FsLib, jobName string, job *Job) (int, error) {
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

func StartMRJob(sc *sigmaclnt.SigmaClnt, jobname string, job *Job, ncoord, nmap, crashtask, crashcoord int, memPerTask proc.Tmem) *groupmgr.GroupMgr {
	cfg := groupmgr.NewGroupConfig(ncoord, "mr-coord", []string{strconv.Itoa(nmap), strconv.Itoa(job.Nreduce), "mr-m-" + job.App, "mr-r-" + job.App, strconv.Itoa(crashtask), strconv.Itoa(job.Linesz), strconv.Itoa(int(memPerTask))}, 0, jobname)
	cfg.SetTest(crashcoord, 0, 0)
	return cfg.StartGrpMgr(sc, ncoord)
}

// XXX run as a proc?
func MergeReducerOutput(fsl *fslib.FsLib, jobName, out string, nreduce int) error {
	file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	wrt := bufio.NewWriter(file)
	for i := 0; i < nreduce; i++ {
		r := strconv.Itoa(i)
		rdr, err := fsl.OpenReader(ReduceOut(jobName) + r + "/")
		if err != nil {
			return err
		}
		if _, err := io.Copy(wrt, rdr); err != nil {
			return err
		}
	}
	return nil
}
