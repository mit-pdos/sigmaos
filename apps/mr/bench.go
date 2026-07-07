package mr

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/dustin/go-humanize"

	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/test"
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
// include proc spawn/queueing delays.
type JobRuntimeStats struct {
	MapInner    *RuntimeStats
	MapOuter    *RuntimeStats
	ReduceInner *RuntimeStats
	ReduceOuter *RuntimeStats
}

func NewJobRuntimeStats(results []*Result) *JobRuntimeStats {
	mInner := []int64{}
	mOuter := []int64{}
	rInner := []int64{}
	rOuter := []int64{}
	for _, r := range results {
		if r.IsM {
			mInner = append(mInner, r.MsInner)
			mOuter = append(mOuter, r.MsOuter)
		} else {
			rInner = append(rInner, r.MsInner)
			rOuter = append(rOuter, r.MsOuter)
		}
	}
	return &JobRuntimeStats{
		MapInner:    newRuntimeStats(mInner),
		MapOuter:    newRuntimeStats(mOuter),
		ReduceInner: newRuntimeStats(rInner),
		ReduceOuter: newRuntimeStats(rOuter),
	}
}

func (jst *JobRuntimeStats) String() string {
	return fmt.Sprintf("mappers  inner: %v\nmappers  outer: %v\nreducers inner: %v\nreducers outer: %v", jst.MapInner, jst.MapOuter, jst.ReduceInner, jst.ReduceOuter)
}

// Read the per-task results the coordinator logged for a job.
func ReadResults(fsl *fslib.FsLib, jobRoot, job string) ([]*Result, error) {
	rdr, err := fsl.OpenReader(MRstats(jobRoot, job))
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(rdr)
	results := []*Result{}
	for {
		r := &Result{}
		if err := dec.Decode(r); err == io.EOF {
			break
		}
		results = append(results, r)
	}
	return results, nil
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
		return test.Tput(results[i].In+results[i].Out, results[i].MsInner) > test.Tput(results[j].In+results[j].Out, results[j].MsInner)
	})
	for _, r := range results {
		fmt.Printf("[%s, kid:%v]:\n\tin %v out %v tot %v inner %vms outer %vms (%s)\n", r.Task, r.KernelID, humanize.Bytes(uint64(r.In)), humanize.Bytes(uint64(r.Out)), test.Mbyte(r.In+r.Out), r.MsInner, r.MsOuter, test.TputStr(r.In+r.Out, r.MsInner))
	}
	fmt.Printf("==== totIn %s (%d) totOut %s tmpOut %s tmpIn %s\n",
		humanize.Bytes(uint64(totIn)), totIn,
		humanize.Bytes(uint64(totOut)),
		humanize.Bytes(uint64(totWTmp)),
		humanize.Bytes(uint64(totRTmp)),
	)
	fmt.Println("==== TASK RUNTIMES:")
	fmt.Println(NewJobRuntimeStats(results))
	return nil
}

func RemoveJob(fsl *fslib.FsLib, jobRoot, job string) error {
	return fsl.RmDir(JobDir(jobRoot, job))
}
