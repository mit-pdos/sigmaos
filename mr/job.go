package mr

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/dustin/go-humanize"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/groupmgr"
	np "ulambda/ninep"
	"ulambda/procclnt"
	"ulambda/test"
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

func shardtarget(job, server, name string, r int) string {
	return np.UX + "/" + server + MR + job + "/m-" + name + "/r-" + strconv.Itoa(r) + "/"
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
	file, err := os.Open(app)
	if err != nil {
		db.DFatalf("ReadConfig err %v\n", err)
	}
	defer file.Close()
	d := yaml.NewDecoder(file)
	if err := d.Decode(&job); err != nil {
		db.DFatalf("Yalm decode %s err %v\n", app, err)
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
		if _, err := fsl.PutFile(n, 0777, np.OWRITE, []byte{}); err != nil {
			db.DFatalf("Putfile %v err %v\n", n, err)
		}
		n = ReduceIn(jobname) + "/" + strconv.Itoa(r)
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Create empty stats file
	if _, err := fsl.PutFile(MRstats(jobname), 0777, np.OWRITE, []byte{}); err != nil {
		db.DFatalf("Putfile %v err %v\n", MRstats(jobname), err)
	}
}

// Put names of input files in name/mr/m
func PrepareJob(fsl *fslib.FsLib, jobName string, job *Job) (int, error) {
	bins, err := MkBins(fsl, job.Input, np.Tlength(job.Binsz))
	if err != nil || len(bins) == 0 {
		return len(bins), err
	}
	for i, b := range bins {
		n := MapTask(jobName) + "/" + BinName(i)
		if _, err := fsl.PutFile(n, 0777, np.OWRITE, []byte{}); err != nil {
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

func StartMRJob(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, jobname string, job *Job, ncoord, nmap, crashtask, crashcoord int) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, ncoord, "user/mr-coord", []string{jobname, strconv.Itoa(nmap), strconv.Itoa(job.Nreduce), "user/mr-m-" + job.App, "user/mr-r-" + job.App, strconv.Itoa(crashtask), strconv.Itoa(job.Linesz)}, ncoord, crashcoord, 0, 0)
}

func MergeReducerOutput(fsl *fslib.FsLib, jobName, out string, nreduce int) error {
	file, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// XXX run as a proc?
	for i := 0; i < nreduce; i++ {
		r := strconv.Itoa(i)
		data, err := fsl.GetFile(ReduceOut(jobName) + r)
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}

func PrintMRStats(fsl *fslib.FsLib, job string) error {
	rdr, err := fsl.OpenReader(MRstats(job))
	if err != nil {
		return err
	}
	dec := json.NewDecoder(rdr)
	fmt.Println("=== STATS:")
	totIn := np.Tlength(0)
	totOut := np.Tlength(0)
	totWTmp := np.Tlength(0)
	totRTmp := np.Tlength(0)
	for {
		var r Result
		if err := dec.Decode(&r); err == io.EOF {
			break
		}
		fmt.Printf("%s: in %s out %s %vms (%s)\n", r.Task, humanize.Bytes(uint64(r.In)), humanize.Bytes(uint64(r.Out)), r.Ms, test.Tput(r.In, r.Ms))
		if r.IsM {
			totIn += r.In
			totWTmp += r.Out
		} else {
			totOut += r.Out
			totRTmp += r.In
		}
	}
	fmt.Printf("=== totIn %s (%d) totOut %s tmpOut %s tmpIn %s\n",
		humanize.Bytes(uint64(totIn)), totIn,
		humanize.Bytes(uint64(totOut)),
		humanize.Bytes(uint64(totWTmp)),
		humanize.Bytes(uint64(totRTmp)),
	)
	return nil
}
