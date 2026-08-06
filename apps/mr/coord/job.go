package coord

import (
	"fmt"
	"strconv"
	"strings"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/ft/task"
	fttask_clnt "sigmaos/ft/task/clnt"
	fttask_srv "sigmaos/ft/task/srv"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/memblock"
)

// Job setup: creating the fttask services a job's tasks live in, submitting
// its tasks, and starting its coordinator. This is driver-side code — it runs
// in the benchmark/test that launches a job, not in a mapper or reducer, which
// is why it lives here rather than in sigmaos/apps/mr: fttask_srv pulls in the
// etcd client and gRPC.

type Tasks struct {
	Mftsrv  *fttask_srv.FtTaskSrvMgr
	Mftclnt fttask_clnt.FtTaskClnt[mr.Bin, any]

	Rftsrv  *fttask_srv.FtTaskSrvMgr
	Rftclnt fttask_clnt.FtTaskClnt[mr.TreduceTask, any]
}

func (ts *Tasks) SubmitReducers(nreducetask int) error {
	rTasks := make([]*fttask_clnt.Task[mr.TreduceTask], nreducetask)
	for r := 0; r < nreducetask; r++ {
		t := mr.TreduceTask{Task: strconv.Itoa(r), Input: nil}
		rTasks[r] = &fttask_clnt.Task[mr.TreduceTask]{Id: fttask_clnt.TaskId(r), Data: t}
	}
	return ts.Rftclnt.SubmitTasks(rTasks)
}

func InitCoordFS(sc *sigmaclnt.SigmaClnt, jobRoot, jobname string, nreducetask int) (*Tasks, error) {
	sc.FsLib.MkDir(mr.MRDIRTOP, 0777)
	sc.FsLib.MkDir(mr.MRDIRELECT, 0777)
	sc.FsLib.MkDir(jobRoot, 0777)

	mftsrv, err := fttask_srv.NewFtTaskSrvMgr(sc, jobname+"-mtasks", false, 1000)
	if err != nil {
		db.DPrintf(db.ERROR, "NewFtTaskSrvMgr %v err %v\n", jobname, err)
		return nil, err
	}
	mftclnt := fttask_clnt.NewFtTaskClnt[mr.Bin, any](sc.FsLib, mftsrv.Id, sp.NullFence())

	rftsrv, err := fttask_srv.NewFtTaskSrvMgr(sc, jobname+"-rtasks", false, 1000)
	if err != nil {
		db.DPrintf(db.ERROR, "NewFtTaskSrvMgr %v err %v\n", jobname, err)
		return nil, err
	}
	rftclnt := fttask_clnt.NewFtTaskClnt[mr.TreduceTask, any](sc.FsLib, rftsrv.Id, sp.NullFence())

	dirs := []string{
		mr.JobDir(jobRoot, jobname),
		mr.LeaderElectDir(jobname),
		mr.MapTask(jobRoot, jobname),
		mr.ReduceTask(jobRoot, jobname),
	}
	for _, n := range dirs {
		if err := sc.FsLib.MkDir(n, 0777); err != nil {
			db.DPrintf(db.ERROR, "Mkdir %v err %v\n", n, err)
			return nil, err
		}
	}
	if err := mr.InitJobSem(sc.FsLib, jobRoot, jobname); err != nil {
		db.DPrintf(db.ERROR, "Err init job sem")
		return nil, err
	}

	return &Tasks{mftsrv, mftclnt, rftsrv, rftclnt}, err
}

func PrepareJob(fsl *fslib.FsLib, ts *Tasks, jobRoot, jobName string, j *mr.Job) (int, error) {
	job := mr.JobLocalToAny(j, false, false, true)
	db.DPrintf(db.TEST, "job %v", job)

	if job.Output == "" || job.Intermediate == "" {
		return 0, fmt.Errorf("Err job output (\"%v\") or intermediate (\"%v\") not supplied", job.Output, job.Intermediate)
	}
	if job.Splitsz == 0 {
		return 0, fmt.Errorf("Err job splitsz not supplied")
	}
	fsl.MkDir(job.Output, 0777)
	outDir := mr.JobOut(job.Output, jobName)
	if err := fsl.MkDir(outDir, 0777); err != nil {
		db.DPrintf(db.ALWAYS, "Error mkdir job dir %v: %v", outDir, err)
		return 0, err
	}
	if _, err := fsl.PutFile(mr.JobOutLink(jobRoot, jobName), 0777, sp.OWRITE, []byte(job.Output)); err != nil {
		db.DPrintf(db.ALWAYS, "Error link output dir [%v] [%v]: %v", job.Output, mr.JobOutLink(jobRoot, jobName), err)
		return 0, err
	}

	// Machines set aside to host this job's data, if any: read here because it
	// decides which UX servers need the intermediate directory, which servers the
	// S3 input is staged to, and which server the input is listed from. Read from
	// the directory memblock procs register themselves in, so nothing has to be
	// threaded in from the caller.
	dedicated, err := memblock.Blocked(fsl)
	if err != nil {
		return 0, fmt.Errorf("PrepareJob: read dedicated UX machines err %v", err)
	}
	if len(dedicated) > 0 {
		db.DPrintf(db.ALWAYS, "PrepareJob: dedicated UX machines %v", dedicated)
	}

	// Create the intermediate output directory before any mapper runs. In S3
	// that is one directory; in UX it is one per server, since each mapper
	// writes its shards to its own. Doing it here keeps mappers from each
	// checking whether it exists, which costs two namespace round trips per
	// mapper — significant with fine-grained mappers.
	if err := mr.CreateIntOutDirsS3(fsl, jobName, job.Intermediate); err != nil {
		db.DPrintf(db.ALWAYS, "CreateIntOutDirsS3 %v err %v", job.Intermediate, err)
		return 0, err
	}
	// Best-effort for UX: on a cold start the UX servers may not be up or known
	// yet, and one may restart during a crash test, so a mapper still creates
	// the directory itself if its server wasn't covered.
	if err := mr.CreateIntOutDirsUx(fsl, jobName, job.Intermediate, dedicated); err != nil {
		db.DPrintf(db.ALWAYS, "CreateIntOutDirsUx %v err %v; mappers will create it themselves", job.Intermediate, err)
	}

	if _, err := fsl.PutFile(mr.JobIntOutLink(jobRoot, jobName), 0777, sp.OWRITE, []byte(job.Intermediate)); err != nil {
		db.DPrintf(db.ALWAYS, "Error link intermediate dir [%v] [%v]: %v", job.Output, mr.JobOutLink(jobRoot, jobName), err)
		return 0, err
	}

	// With dedicated machines the input has to be listed from one of them: it may
	// exist only there, and ~any would land on whichever server named answered,
	// most likely one with nothing in it — an empty bin list, which reads as "the
	// dataset was never staged". Only the *listing* names a dedicated machine; the
	// splits keep ~local, so the coordinator can hand each mapper a different
	// dedicated server (see Coord.mapperProc).
	listElem := sp.ANY
	if len(dedicated) > 0 {
		if err := mr.CheckInputOnUxSrvs(fsl, job.Input, dedicated); err != nil {
			return 0, err
		}
		listElem = dedicated[0]
	}

	bins, err := mr.NewBins(fsl, job.Input, listElem, sp.Tlength(job.Binsz), sp.Tlength(job.Splitsz))
	if err != nil || len(bins) == 0 {
		if strings.Contains(job.Input, sp.INPUT_DATA_REL) {
			// The job reads pre-staged input, which start-kernel.sh bind-mounts
			// into each kernel container from the host. An empty or missing
			// directory almost always means the dataset was never staged.
			db.DPrintf(db.ALWAYS, "No input at %v (%d bins, err %v): stage the dataset on every node first (./download-input-data.sh <dataset>, or --input-data on the cluster start script)", job.Input, len(bins), err)
		}
		return len(bins), err
	}
	mtasks := make([]*fttask_clnt.Task[mr.Bin], len(bins))
	for i, b := range bins {
		mtasks[i] = &fttask_clnt.Task[mr.Bin]{Id: fttask_clnt.TaskId(i), Data: b}
	}
	err = ts.Mftclnt.SubmitTasks(mtasks)
	return len(bins), err
}

// mapperMem and reducerMem are the memory each mapper and each reducer
// reserves, which is what bounds how many of them run concurrently per node.
// They are separate because the phases want different packings: mappers are
// throughput-bound and want to pack densely, while a reducer reads every
// mapper's shard and contends for CPU and network with anything sharing its
// node, so spreading reducers out can matter more than packing them.
func StartMRJob(sc *sigmaclnt.SigmaClnt, jobRoot, jobName string, job *mr.Job, nmap int, mapperMem, reducerMem proc.Tmem, maliciousMapper int, mftid task.FtTaskSvcId, rftid task.FtTaskSvcId) *procgroupmgr.ProcGroupMgr {
	cfg := procgroupmgr.NewProcGroupConfig(NCOORD, "mr-coord",
		[]string{
			jobRoot,
			strconv.Itoa(nmap),
			strconv.Itoa(job.Nreduce),
			"mr-m-" + job.App,
			"mr-r-" + job.App,
			strconv.Itoa(job.Linesz),
			strconv.Itoa(job.Wordsz),
			strconv.Itoa(int(mapperMem)),
			strconv.Itoa(maliciousMapper),
			string(mftid),
			string(rftid),
			strconv.FormatBool(job.UseGetPut),
			strconv.FormatBool(job.UseCosandboxes),
			strconv.Itoa(job.TailProbeSz),
			strconv.Itoa(job.MapperGOMAXPROCS),
			strconv.Itoa(job.Binsz),
			strconv.FormatBool(job.UseGetPutReduce),
			strconv.FormatBool(job.UseCosandboxesReduce),
			strconv.Itoa(job.ReduceShmemMB),
			// Appended rather than placed next to mapperMem above, so that adding
			// it did not renumber the coordinator's other argument indices.
			strconv.Itoa(int(reducerMem)),
			strconv.Itoa(job.ReduceGetsConcurrency),
		}, 1000, jobName)
	return cfg.StartGrpMgr(sc)
}
