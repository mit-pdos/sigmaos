package mr

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/groupmgr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/yaml"
)

func JobDir(job string) string {
	return MRDIRTOP + "/" + job
}

func MRstats(job string) string {
	return JobDir(job) + "/stats.txt"
}

func MapTask(job string) string {
	return JobDir(job) + "/m"
}

func ReduceTask(job string) string {
	return JobDir(job) + "/r"
}

func ReduceIn(job string) string {
	return JobDir(job) + "-rin/"
}

func ReduceOut(job string) string {
	return JobDir(job) + "/mr-out-"
}

func BinName(i int) string {
	return fmt.Sprintf("bin%04d", i)
}

func LocalOut(job string) string {
	return MLOCALDIR + "/" + job + "/"
}

func Moutdir(job, name string) string {
	return LocalOut(job) + "m-" + name
}

func mshardfile(job, name string, r int) string {
	return Moutdir(job, name) + "/r-" + strconv.Itoa(r)
}

func shardtarget(job, pn, name string, r int) string {
	return pn + MR + job + "/m-" + name + "/r-" + strconv.Itoa(r) + "/"
}

func symname(job, r, name string) string {
	return ReduceIn(job) + "/" + r + "/m-" + name
}

type Job struct {
	App     string `yalm:"app"`
	Nreduce int    `yalm:"nreduce"`
	Binsz   int    `yalm:"binsz"`
	Input   string `yalm:"input"`
	Linesz  int    `yalm:"linesz"`
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
	for _, n := range []string{JobDir(jobname), MapTask(jobname), ReduceTask(jobname), ReduceIn(jobname), MapTask(jobname) + TIP, ReduceTask(jobname) + TIP, MapTask(jobname) + DONE, ReduceTask(jobname) + DONE, MapTask(jobname) + NEXT, ReduceTask(jobname) + NEXT} {
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

// Put names of input files in name/mr/m
func PrepareJob(fsl *fslib.FsLib, jobName string, job *Job) (int, error) {
	splitsz := sp.Tlength(10 * sp.MBYTE)
	// splitsz := maxbinsz >> 3 //sp.Tlength(10 * 1024 * 1024)

	bins, err := MkBins(fsl, job.Input, sp.Tlength(job.Binsz), splitsz)
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

func StartMRJob(sc *sigmaclnt.SigmaClnt, jobname string, job *Job, ncoord, nmap, crashtask, crashcoord int) *groupmgr.GroupMgr {
	cfg := groupmgr.NewGroupConfig(sc, ncoord, "mr-coord", []string{strconv.Itoa(nmap), strconv.Itoa(job.Nreduce), "mr-m-" + job.App, "mr-r-" + job.App, strconv.Itoa(crashtask), strconv.Itoa(job.Linesz)}, 0, jobname)
	cfg.SetTest(crashcoord, 0, 0)
	return cfg.Start(ncoord)
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
		rdr, err := fsl.OpenReader(ReduceOut(jobName) + r)
		if err != nil {
			return err
		}
		if _, err := io.Copy(wrt, rdr); err != nil {
			return err
		}
	}
	return nil
}
