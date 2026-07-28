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
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/coordination/semaphore"
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
	// If > 0, GOMAXPROCS for mapper procs. Unset (0) leaves the Go default,
	// which is the machine's core count regardless of how many procs share
	// the machine — see claude-slop/SLOW_MAPPER_EXEC.md.
	MapperGOMAXPROCS int `json:"mapper_gomaxprocs,omitempty"`
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

// CreateIntOutDirsUx creates the job's intermediate output directory on every
// UX server, once, before any mapper runs. Called from job preparation
// (coord.PrepareJob), alongside the equivalent for S3 intermediate output.
//
// Mappers used to each create it themselves (CreateMapperIntOutDirUx below),
// which costs two namespace round trips per mapper to learn that the directory
// already exists — every mapper after the first on a node. With fine-grained
// mappers that is a real fraction of a mapper's CPU (measured at ~5 ms of the
// ~34 ms a mapper spends, of which only ~7 ms is the mapping itself; see
// claude-slop/SLOW_MAPPER_EXEC.md). Doing it once per server per job instead
// makes it ~free.
//
// Idempotent, so concurrent coordinators (or a re-run job) are fine.
func CreateIntOutDirsUx(fsl *fslib.FsLib, job, intOutput string) error {
	if !strings.Contains(intOutput, "/ux/") {
		// S3 intermediate output: PrepareJob creates those directories once.
		return nil
	}
	// A ~local intermediate path means "each mapper's own UX server", so the
	// directory has to exist on all of them. Any other path names one server.
	if !sp.HasLocal(intOutput) {
		return mkDirsIntOut(fsl, job, intOutput)
	}
	sts, err := fsl.GetDir(sp.UX)
	if err != nil {
		db.DPrintf(db.ERROR, "CreateIntOutDirsUx GetDir %v err %v", sp.UX, err)
		return err
	}
	srvs := sp.Names(sts)
	db.DPrintf(db.MR, "CreateIntOutDirsUx %v on %d UX srvs", intOutput, len(srvs))
	errc := make(chan error, len(srvs))
	for _, srv := range srvs {
		go func(srv string) {
			pn, _ := sp.SubstLocal(intOutput, srv)
			errc <- mkDirsIntOut(fsl, job, pn)
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

// mkDirsIntOut creates intOutput and intOutput/<job>, tolerating both already
// existing. MkDir-and-ignore-exists rather than Stat-then-MkDir: one round trip
// instead of two, and it races correctly against another creator.
func mkDirsIntOut(fsl *fslib.FsLib, job, intOutput string) error {
	for _, pn := range []string{intOutput, MapIntermediateDir(job, intOutput)} {
		if err := fsl.MkDir(pn, 0777); err != nil && !serr.IsErrorExists(err) {
			db.DPrintf(db.ERROR, "MkDir %v err %v", pn, err)
			return err
		}
	}
	return nil
}

// CreateMapperIntOutDirUx creates the intermediate output directory a mapper
// writes its shards to. Job preparation normally creates it on every UX server
// before the mappers run (CreateIntOutDirsUx), so this is only the fallback for
// a server that wasn't covered — one that wasn't up or known at preparation
// time (cold start), or that restarted since (UX crash tests).
func CreateMapperIntOutDirUx(fsl *fslib.FsLib, job, intOutput string) error {
	if !strings.Contains(intOutput, "/ux/") {
		return nil
	}
	return mkDirsIntOut(fsl, job, intOutput)
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
