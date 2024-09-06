package mr

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/dustin/go-humanize"

	"sigmaos/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func PrintMRStats(fsl *fslib.FsLib, job string) error {
	rdr, err := fsl.OpenReader(MRstats(job))
	if err != nil {
		return err
	}
	dec := json.NewDecoder(rdr.Reader)
	fmt.Println("=== STATS:")
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
		return test.Tput(results[i].In+results[i].Out, results[i].Ms) > test.Tput(results[j].In+results[j].Out, results[j].Ms)
	})
	for _, r := range results {
		fmt.Printf("%s: in %s out tot %v %f %vms (%s)\n", r.Task, humanize.Bytes(uint64(r.In)), humanize.Bytes(uint64(r.Out)), test.Mbyte(r.In+r.Out), r.Ms, test.TputStr(r.In+r.Out, r.Ms))
	}
	fmt.Printf("=== totIn %s (%d) totOut %s tmpOut %s tmpIn %s\n",
		humanize.Bytes(uint64(totIn)), totIn,
		humanize.Bytes(uint64(totOut)),
		humanize.Bytes(uint64(totWTmp)),
		humanize.Bytes(uint64(totRTmp)),
	)
	return nil
}

func RemoveJob(fsl *fslib.FsLib, job string) error {
	return fsl.RmDir(JobDir(job))
}
