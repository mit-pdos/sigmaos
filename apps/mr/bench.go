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

func PrintMRStats(fsl *fslib.FsLib, jobRoot, job string) error {
	rdr, err := fsl.OpenReader(MRstats(jobRoot, job))
	if err != nil {
		return err
	}
	dec := json.NewDecoder(rdr)
	fmt.Println("==== STATS:")
	totIn := sp.Tlength(0)
	totOut := sp.Tlength(0)
	totWTmp := sp.Tlength(0)
	totRTmp := sp.Tlength(0)
	results := []*Result{}
	for {
		r := &Result{}
		if err := dec.Decode(r); err == io.EOF {
			break
		}
		results = append(results, r)
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
	// Per-task overhead (MsOuter-MsInner) is a proxy for admission-slot
	// pressure: time spent waiting for a slot on top of actual compute.
	// Aggregate it split by phase.
	var mOverTot, rOverTot, mOverMax, rOverMax int64
	var nM, nR int
	for _, r := range results {
		over := r.MsOuter - r.MsInner
		if over < 0 {
			over = 0
		}
		if r.IsM {
			mOverTot += over
			nM++
			mOverMax = max(mOverMax, over)
		} else {
			rOverTot += over
			nR++
			rOverMax = max(rOverMax, over)
		}
		fmt.Printf("[%s, kid:%v]:\n\tin %v out %v tot %v inner %vms outer %vms (%s)\n", r.Task, r.KernelID, humanize.Bytes(uint64(r.In)), humanize.Bytes(uint64(r.Out)), test.Mbyte(r.In+r.Out), r.MsInner, r.MsOuter, test.TputStr(r.In+r.Out, r.MsInner))
	}
	fmt.Printf("==== totIn %s (%d) totOut %s tmpOut %s tmpIn %s\n",
		humanize.Bytes(uint64(totIn)), totIn,
		humanize.Bytes(uint64(totOut)),
		humanize.Bytes(uint64(totWTmp)),
		humanize.Bytes(uint64(totRTmp)),
	)
	mOverMean, rOverMean := int64(0), int64(0)
	if nM > 0 {
		mOverMean = mOverTot / int64(nM)
	}
	if nR > 0 {
		rOverMean = rOverTot / int64(nR)
	}
	fmt.Printf("==== slot pressure (outer-inner queueing overhead): map tot %dms mean %dms max %dms (n=%d); reduce tot %dms mean %dms max %dms (n=%d)\n",
		mOverTot, mOverMean, mOverMax, nM, rOverTot, rOverMean, rOverMax, nR)
	return nil
}

func RemoveJob(fsl *fslib.FsLib, jobRoot, job string) error {
	return fsl.RmDir(JobDir(jobRoot, job))
}
