package mr

import (
	"bufio"
	"encoding/json"
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

func MRPhaseStats(jobRoot, job string) string {
	return filepath.Join(JobDir(jobRoot, job), "phases.txt")
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
	App          string `json:"app"`
	Nreduce      int    `json:"nreduce"`
	Binsz        int    `json:"binsz"`
	Splitsz      int    `json:"splitsz"`
	Input        string `json:"input"`
	Intermediate string `json:"intermediate"`
	Output       string `json:"output"`
	Linesz       int    `json:"linesz"`
	Wordsz       int    `json:"wordsz"`
	Local        string `json:"local,omitempty"`
	// Optional S3 source for the job's input. If set (and the job's input is
	// stored in UX), the input files are copied from the S3 source to the
	// job's input directory on every UX server during job setup.
	S3Input string `json:"s3input,omitempty"`
	// Mappers read input/write intermediate output through the UX/S3 proxy
	// Get/Put client API (proxy/getput) instead of the fslib streaming
	// reader/writer.
	UseGetPut bool `json:"use_getput,omitempty"`
	// A cosandbox pre-fetches each mapper's input splits before the mapper
	// starts (requires UseGetPut).
	UseCosandboxes bool `json:"use_cosandboxes,omitempty"`
	// Initial size of the tail probe read past each split's end on the
	// getput path (0 = mr.DEFAULT_TAIL_PROBE_SZ; capped at Linesz).
	TailProbeSz int `json:"tailprobesz,omitempty"`
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
	b, err := os.ReadFile(app)
	if err != nil {
		db.DPrintf(db.ERROR, "ReadConfig err %v\n", err)
		return nil, err
	}
	job := &Job{}
	if err := json.Unmarshal(b, job); err != nil {
		db.DPrintf(db.ERROR, "ReadConfig unmarshal err %v\n", err)
		return nil, err
	}
	if job.Splitsz == 0 {
		return nil, fmt.Errorf("Err job %v has no splitsz", app)
	}
	if job.UseCosandboxes && !job.UseGetPut {
		return nil, fmt.Errorf("Err job %v: use_cosandboxes requires use_getput", app)
	}
	return job, nil
}

type Tasks struct {
	Mftsrv  *fttask_srv.FtTaskSrvMgr
	Mftclnt fttask_clnt.FtTaskClnt[Bin, any]

	Rftsrv  *fttask_srv.FtTaskSrvMgr
	Rftclnt fttask_clnt.FtTaskClnt[TreduceTask, any]
}

func (ts *Tasks) SubmitReducers(nreducetask int) error {
	rTasks := make([]*fttask_clnt.Task[TreduceTask], nreducetask)
	for r := 0; r < nreducetask; r++ {
		t := TreduceTask{strconv.Itoa(r), nil}
		rTasks[r] = &fttask_clnt.Task[TreduceTask]{Id: fttask_clnt.TaskId(r), Data: t}
	}
	return ts.Rftclnt.SubmitTasks(rTasks)
}

func InitCoordFS(sc *sigmaclnt.SigmaClnt, jobRoot, jobname string, nreducetask int) (*Tasks, error) {
	sc.FsLib.MkDir(MRDIRTOP, 0777)
	sc.FsLib.MkDir(MRDIRELECT, 0777)
	sc.FsLib.MkDir(jobRoot, 0777)

	mftsrv, err := fttask_srv.NewFtTaskSrvMgr(sc, jobname+"-mtasks", false, 1000)
	if err != nil {
		db.DPrintf(db.ERROR, "NewFtTaskSrvMgr %v err %v\n", jobname, err)
		return nil, err
	}
	mftclnt := fttask_clnt.NewFtTaskClnt[Bin, any](sc.FsLib, mftsrv.Id, sp.NullFence())

	rftsrv, err := fttask_srv.NewFtTaskSrvMgr(sc, jobname+"-rtasks", false, 1000)
	if err != nil {
		db.DPrintf(db.ERROR, "NewFtTaskSrvMgr %v err %v\n", jobname, err)
		return nil, err
	}
	rftclnt := fttask_clnt.NewFtTaskClnt[TreduceTask, any](sc.FsLib, rftsrv.Id, sp.NullFence())

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

	return &Tasks{mftsrv, mftclnt, rftsrv, rftclnt}, err
}

// Clean up all old MR outputs
func CleanupMROutputs(fsl *fslib.FsLib, outputDir, intOutputDir string, swapLocalForAny bool) error {
	db.DPrintf(db.MR, "Clean up MR outputs: %v %v", outputDir, intOutputDir)
	defer db.DPrintf(db.MR, "Clean up MR outputs done")

	fsl.RmDir(intOutputDir)
	oDir := outputDir
	if swapLocalForAny {
		oDir, _ = sp.SubstLocal(oDir, sp.ANY)
	}
	return fsl.RmDir(oDir)
}

func JobLocalToAny(j *Job, input, intermediate, output bool) *Job {
	// Make a copy of the job struct so we can adjust some paths (e.g., replace
	// ~local with ~any), for the test program
	job := &Job{}
	*job = *j
	if input {
		job.Input, _ = sp.SubstLocal(job.Input, sp.ANY)
	}
	if intermediate {
		job.Intermediate, _ = sp.SubstLocal(job.Intermediate, sp.ANY)
	}
	if output {
		job.Output, _ = sp.SubstLocal(job.Output, sp.ANY)
	}
	return job
}

// If the job specifies an S3 input source, copy the job's input files from
// S3 to the job's input directory on every UX server, so mappers can read
// their input from UX. Files which already exist on a UX server (with the
// expected length) are not copied again (e.g., if they were already copied
// for another realm's job).
func CopyS3InputToUx(fsl *fslib.FsLib, j *Job) error {
	if j.S3Input == "" {
		return nil
	}
	if !strings.HasPrefix(j.Input, sp.UX) {
		return fmt.Errorf("job has S3 input source %v, but input %v is not stored in UX", j.S3Input, j.Input)
	}
	// Strip the UX mount prefix and server selector (e.g., ~local) off of the
	// input path, to get the input path relative to each UX server's root
	p := strings.SplitN(strings.TrimPrefix(j.Input, sp.UX), "/", 2)
	if len(p) != 2 || p[1] == "" {
		return fmt.Errorf("no input dir in UX input path %v", j.Input)
	}
	inputRelPath := p[1]
	srvs, err := fsl.GetDir(sp.UX)
	if err != nil {
		db.DPrintf(db.ERROR, "GetDir %v err %v", sp.UX, err)
		return err
	}
	inputs, err := fsl.GetDir(j.S3Input)
	if err != nil {
		db.DPrintf(db.ERROR, "GetDir %v err %v", j.S3Input, err)
		return err
	}
	db.DPrintf(db.MR, "Copy S3 input %v to %v on %d UX srvs", j.S3Input, j.Input, len(srvs))
	defer db.DPrintf(db.MR, "Done copy S3 input %v to %v on %d UX srvs", j.S3Input, j.Input, len(srvs))
	errc := make(chan error, len(srvs))
	for _, srv := range sp.Names(srvs) {
		go func(srv string) {
			errc <- copyS3InputToUxSrv(fsl, j.S3Input, inputs, filepath.Join(sp.UX, srv), inputRelPath)
		}(srv)
	}
	var err1 error
	for range srvs {
		if err := <-errc; err != nil {
			err1 = err
		}
	}
	return err1
}

// Copy the job's input files from S3 to a UX server.
func copyS3InputToUxSrv(fsl *fslib.FsLib, s3Input string, inputs []*sp.Tstat, uxSrv, inputRelPath string) error {
	if err := fsl.MkDirPath(uxSrv, inputRelPath, 0777); err != nil && !serr.IsErrorExists(err) {
		db.DPrintf(db.ERROR, "MkDirPath %v/%v err %v", uxSrv, inputRelPath, err)
		return err
	}
	dstDir := filepath.Join(uxSrv, inputRelPath)
	for _, st := range inputs {
		src := filepath.Join(s3Input, st.Name)
		dst := filepath.Join(dstDir, st.Name)
		// Skip files which were already copied to this UX server
		if dstSt, err := fsl.Stat(dst); err == nil && dstSt.Tlength() == st.Tlength() {
			continue
		}
		db.DPrintf(db.MR, "Copy input %v -> %v", src, dst)
		if err := fsl.CopyFile(src, dst); err != nil {
			db.DPrintf(db.ERROR, "CopyFile %v -> %v err %v", src, dst, err)
			return err
		}
	}
	return nil
}

func PrepareJob(fsl *fslib.FsLib, ts *Tasks, jobRoot, jobName string, j *Job) (int, error) {
	job := JobLocalToAny(j, false, false, true)
	db.DPrintf(db.TEST, "job %v", job)

	if job.Output == "" || job.Intermediate == "" {
		return 0, fmt.Errorf("Err job output (\"%v\") or intermediate (\"%v\") not supplied", job.Output, job.Intermediate)
	}
	if job.Splitsz == 0 {
		return 0, fmt.Errorf("Err job splitsz not supplied")
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

	bins, err := NewBins(fsl, job.Input, true, sp.Tlength(job.Binsz), sp.Tlength(job.Splitsz))
	if err != nil || len(bins) == 0 {
		return len(bins), err
	}
	mtasks := make([]*fttask_clnt.Task[Bin], len(bins))
	for i, b := range bins {
		mtasks[i] = &fttask_clnt.Task[Bin]{Id: fttask_clnt.TaskId(i), Data: b}
	}
	err = ts.Mftclnt.SubmitTasks(mtasks)
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

func StartMRJob(sc *sigmaclnt.SigmaClnt, jobRoot, jobName string, job *Job, nmap int, memPerTask proc.Tmem, maliciousMapper int, mftid task.FtTaskSvcId, rftid task.FtTaskSvcId) *procgroupmgr.ProcGroupMgr {
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
			strconv.FormatBool(job.UseGetPut),
			strconv.FormatBool(job.UseCosandboxes),
			strconv.Itoa(job.TailProbeSz),
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
