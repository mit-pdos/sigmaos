package mr

import (
	"fmt"
	"os"
	"strconv"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
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

func InitCoordFS(fsl *fslib.FsLib, job string, nreducetask int) {
	fsl.MkDir(MRDIRTOP, 0777)
	for _, n := range []string{JobDir(job), MapTask(job), ReduceTask(job), ReduceIn(job), MapTask(job) + TIP, ReduceTask(job) + TIP, MapTask(job) + DONE, ReduceTask(job) + DONE, MapTask(job) + NEXT, ReduceTask(job) + NEXT} {
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Make task and input directories for reduce tasks
	for r := 0; r < nreducetask; r++ {
		n := ReduceTask(job) + "/" + strconv.Itoa(r)
		if _, err := fsl.PutFile(n, 0777, np.OWRITE, []byte{}); err != nil {
			db.DFatalf("Putfile %v err %v\n", n, err)
		}
		n = ReduceIn(job) + "/" + strconv.Itoa(r)
		if err := fsl.MkDir(n, 0777); err != nil {
			db.DFatalf("Mkdir %v err %v\n", n, err)
		}
	}

	// Create empty stats file
	if _, err := fsl.PutFile(MRstats(job), 0777, np.OWRITE, []byte{}); err != nil {
		db.DFatalf("Putfile %v err %v\n", MRstats(job), err)
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
