package coord

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/dustin/go-humanize"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/tput"
)

// Summary statistics for a set of task runtimes (in ms).
type RuntimeStats struct {
	N        int
	MinMs    int64
	MaxMs    int64
	MeanMs   float64
	MedianMs int64
	P90Ms    int64
}

func newRuntimeStats(ms []int64) *RuntimeStats {
	st := &RuntimeStats{N: len(ms)}
	if len(ms) == 0 {
		return st
	}
	sorted := make([]int64, len(ms))
	copy(sorted, ms)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	sum := int64(0)
	for _, m := range sorted {
		sum += m
	}
	st.MinMs = sorted[0]
	st.MaxMs = sorted[len(sorted)-1]
	st.MeanMs = float64(sum) / float64(len(sorted))
	st.MedianMs = percentile(sorted, 50)
	st.P90Ms = percentile(sorted, 90)
	return st
}

// Compute the pth percentile of a sorted slice of runtimes
func percentile(sorted []int64, p int) int64 {
	idx := (p*len(sorted)+99)/100 - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (st *RuntimeStats) String() string {
	if st.N == 0 {
		return "n 0"
	}
	return fmt.Sprintf("n %d min %vms mean %.1fms median %vms p90 %vms max %vms", st.N, st.MinMs, st.MeanMs, st.MedianMs, st.P90Ms, st.MaxMs)
}

// Runtime statistics for a job's mappers and reducers. Inner runtimes are
// measured within the mapper/reducer procs themselves, and only include task
// execution time. Outer runtimes are measured at the coordinator, and also
// include proc spawn/queueing delays. Get times are the part of the inner
// runtime a task spent fetching its input (see mr.Result.MsGet); on a mapper's
// fslib path they can exceed it, since chunk reads run concurrently.
type JobRuntimeStats struct {
	MapInner    *RuntimeStats
	MapOuter    *RuntimeStats
	MapGet      *RuntimeStats
	ReduceInner *RuntimeStats
	ReduceOuter *RuntimeStats
	ReduceGet   *RuntimeStats
}

func NewJobRuntimeStats(results []*mr.Result) *JobRuntimeStats {
	mInner := []int64{}
	mOuter := []int64{}
	mGet := []int64{}
	rInner := []int64{}
	rOuter := []int64{}
	rGet := []int64{}
	for _, r := range results {
		if r.IsM {
			mInner = append(mInner, r.MsInner)
			mOuter = append(mOuter, r.MsOuter)
			mGet = append(mGet, r.MsGet)
		} else {
			rInner = append(rInner, r.MsInner)
			rOuter = append(rOuter, r.MsOuter)
			rGet = append(rGet, r.MsGet)
		}
	}
	return &JobRuntimeStats{
		MapInner:    newRuntimeStats(mInner),
		MapOuter:    newRuntimeStats(mOuter),
		MapGet:      newRuntimeStats(mGet),
		ReduceInner: newRuntimeStats(rInner),
		ReduceOuter: newRuntimeStats(rOuter),
		ReduceGet:   newRuntimeStats(rGet),
	}
}

func (jst *JobRuntimeStats) String() string {
	return fmt.Sprintf("mappers  inner: %v\nmappers  outer: %v\nmappers  gets:  %v\nreducers inner: %v\nreducers outer: %v\nreducers gets:  %v",
		jst.MapInner, jst.MapOuter, jst.MapGet, jst.ReduceInner, jst.ReduceOuter, jst.ReduceGet)
}

// Read the per-task results the coordinator logged for a job.
func ReadResults(fsl *fslib.FsLib, jobRoot, job string) ([]*mr.Result, error) {
	rdr, err := fsl.OpenReader(mr.MRstats(jobRoot, job))
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(rdr)
	results := []*mr.Result{}
	for {
		r := &mr.Result{}
		if err := dec.Decode(r); err == io.EOF {
			break
		}
		results = append(results, r)
	}
	return results, nil
}

// Read the map/reduce phase durations the coordinator logged for a job.
func ReadPhaseDurations(fsl *fslib.FsLib, jobRoot, job string) (*mr.PhaseDurations, error) {
	pd := &mr.PhaseDurations{}
	if err := fsl.GetFileJson(mr.MRPhaseStats(jobRoot, job), pd); err != nil {
		return nil, err
	}
	return pd, nil
}

func PrintMRStats(fsl *fslib.FsLib, jobRoot, job string) error {
	results, err := ReadResults(fsl, jobRoot, job)
	if err != nil {
		return err
	}
	fmt.Println("==== STATS:")
	totIn := sp.Tlength(0)
	totOut := sp.Tlength(0)
	totWTmp := sp.Tlength(0)
	totRTmp := sp.Tlength(0)
	for _, r := range results {
		if r.IsM {
			totIn += r.In
			totWTmp += r.Out
		} else {
			totOut += r.Out
			totRTmp += r.In
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return tput.Tput(results[i].In+results[i].Out, results[i].MsInner) > tput.Tput(results[j].In+results[j].Out, results[j].MsInner)
	})
	for _, r := range results {
		gets := fmt.Sprintf(" gets %vms (n %v)", r.MsGet, r.NGet)
		fmt.Printf("[%s, kid:%v]:\n\tin %v out %v tot %v inner %vms outer %vms%s (%s)\n", r.Task, r.KernelID, humanize.Bytes(uint64(r.In)), humanize.Bytes(uint64(r.Out)), tput.Mbyte(r.In+r.Out), r.MsInner, r.MsOuter, gets, tput.TputStr(r.In+r.Out, r.MsInner))
	}
	fmt.Printf("==== totIn %s (%d) totOut %s tmpOut %s tmpIn %s\n",
		humanize.Bytes(uint64(totIn)), totIn,
		humanize.Bytes(uint64(totOut)),
		humanize.Bytes(uint64(totWTmp)),
		humanize.Bytes(uint64(totRTmp)),
	)
	fmt.Println("==== TASK RUNTIMES:")
	fmt.Println(NewJobRuntimeStats(results))
	if pd, err := ReadPhaseDurations(fsl, jobRoot, job); err != nil {
		db.DPrintf(db.MR, "ReadPhaseDurations err %v", err)
	} else {
		fmt.Println("==== PHASE DURATIONS:")
		fmt.Printf("map phase %vms reduce phase %vms\n", pd.MapMs, pd.ReduceMs)
	}
	return nil
}

func RemoveJob(fsl *fslib.FsLib, jobRoot, job string) error {
	return fsl.RmDir(mr.JobDir(jobRoot, job))
}
