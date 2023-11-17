package imgresized

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leaderclnt"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	IMG  = "name/img"
	STOP = "_STOP"
)

type ImgSrv struct {
	*sigmaclnt.SigmaClnt
	wg         sync.WaitGroup
	job        string
	done       string
	wip        string
	todo       string
	nrounds    int
	workerMcpu proc.Tmcpu
	workerMem  proc.Tmem
	crash      int64
	exited     bool
	leaderclnt *leaderclnt.LeaderClnt
}

func MkDirs(fsl *fslib.FsLib, job string) error {
	fsl.RmDir(IMG)
	if err := fsl.MkDir(IMG, 0777); err != nil {
		return err
	}
	if err := fsl.MkDir(path.Join(IMG, job), 0777); err != nil {
		return err
	}
	if err := fsl.MkDir(path.Join(IMG, job, "done"), 0777); err != nil {
		return err
	}
	if err := fsl.MkDir(path.Join(IMG, job, "todo"), 0777); err != nil {
		return err
	}
	if err := fsl.MkDir(path.Join(IMG, job, "wip"), 0777); err != nil {
		return err
	}
	return nil
}

func SubmitTask(fsl *fslib.FsLib, job string, fn string) error {
	return SubmitTaskMulti(fsl, job, []string{fn})
}

func SubmitTaskMulti(fsl *fslib.FsLib, job string, fns []string) error {
	t := path.Join(sp.IMG, job, "todo", rd.String(4))
	_, err := fsl.PutFile(t, 0777, sp.OREAD, []byte(strings.Join(fns, ",")))
	return err
}

func NTaskDone(fsl *fslib.FsLib, job string) (int, error) {
	sts, err := fsl.GetDir(path.Join(sp.IMG, job, "done"))
	if err != nil {
		return -1, err
	}
	return len(sts), nil
}

func Cleanup(fsl *fslib.FsLib, dir string) error {
	_, err := fsl.ProcessDir(dir, func(st *sp.Stat) (bool, error) {
		if strings.Contains(st.Name, "thumb") {
			err := fsl.Remove(path.Join(dir, st.Name))
			if err != nil {
				return true, err
			}
			return false, nil
		}
		return false, nil
	})
	return err
}

func NewImgd(args []string) (*ImgSrv, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("NewImgSrv: wrong number of arguments: %v", args)
	}
	imgd := &ImgSrv{}
	imgd.job = args[0]
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.IMGD, "Made fslib job %v", imgd.job)
	imgd.SigmaClnt = sc
	imgd.job = args[0]
	crashing, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: error parse crash %v", err)
	}
	imgd.crash = int64(crashing)
	imgd.done = path.Join(IMG, imgd.job, "done")
	imgd.todo = path.Join(IMG, imgd.job, "todo")
	imgd.wip = path.Join(IMG, imgd.job, "wip")
	mcpu, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse MCPU %v", err)
	}
	imgd.workerMcpu = proc.Tmcpu(mcpu)
	mem, err := strconv.Atoi(args[3])
	if err != nil {
		return nil, fmt.Errorf("NewImgSrv: Error parse Mem %v", err)
	}
	imgd.workerMem = proc.Tmem(mem)
	imgd.nrounds, err = strconv.Atoi(args[4])
	if err != nil {
		db.DFatalf("Error parse nrounds: %v", err)
	}

	imgd.Started()

	imgd.leaderclnt, err = leaderclnt.NewLeaderClnt(imgd.FsLib, path.Join(IMG, imgd.job, "imgd-leader"), 0777)
	if err != nil {
		return nil, fmt.Errorf("NewLeaderclnt err %v", err)
	}

	crash.Crasher(imgd.FsLib)

	go func() {
		imgd.WaitEvict(sc.ProcEnv().GetPID())
		if !imgd.exited {
			imgd.ClntExitOK()
		}
		os.Exit(0)
	}()

	return imgd, nil
}

func (imgd *ImgSrv) claimEntry(name string) (string, error) {
	if err := imgd.Rename(imgd.todo+"/"+name, imgd.wip+"/"+name); err != nil {
		if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
			return "", err
		}
		// another thread claimed the task before us
		db.DPrintf(db.IMGD, "Error claim entry %v: %v", name, err)
		return "", nil
	}
	db.DPrintf(db.IMGD, "Claim %v success", name)
	return name, nil
}

type task struct {
	name string
	fn   string
}

type Tresult struct {
	t   string
	ok  bool
	ms  int64
	msg string
}

func (imgd *ImgSrv) waitForTask(start time.Time, p *proc.Proc, t *task) Tresult {
	imgd.WaitStart(p.GetPid())
	db.DPrintf(db.ALWAYS, "Start Latency %v", time.Since(start))
	status, err := imgd.WaitExit(p.GetPid())
	ms := time.Since(start).Milliseconds()
	if err == nil && status.IsStatusOK() {
		// mark task as done
		if err := imgd.Rename(imgd.wip+"/"+t.name, imgd.done+"/"+t.name); err != nil {
			db.DFatalf("rename task %v done err %v", t, err)
		}
		return Tresult{t.name, true, ms, status.Msg()}
	} else if err == nil && status.IsStatusErr() {
		db.DPrintf(db.ALWAYS, "task %v errored err %v", t, status)
		// mark task as done, but return error
		if err := imgd.Rename(imgd.wip+"/"+t.name, imgd.done+"/"+t.name); err != nil {
			db.DFatalf("rename task %v done err %v", t, err)
		}
		return Tresult{t.name, false, ms, status.Msg()}
	} else { // task failed; make it runnable again
		db.DPrintf(db.IMGD, "task %v failed %v err %v", t, status, err)
		if err := imgd.Rename(imgd.wip+"/"+t.name, imgd.todo+"/"+t.name); err != nil {
			db.DFatalf("rename task %v todo err %v", t, err)
		}
		return Tresult{t.name, false, ms, ""}
	}
}

func ThumbName(fn string) string {
	ext := path.Ext(fn)
	fn1 := strings.TrimSuffix(fn, ext) + "-" + rd.String(4) + "-thumb" + path.Ext(fn)
	return fn1
}

func (imgd *ImgSrv) runTask(t *task) {
	// Mark the task as done, regardless of whether it succeeded or not. If it
	// didn't succeed, it will be added to the work queue again, which will
	// result in another wg.Add
	defer imgd.wg.Done()

	p := proc.NewProcPid(sp.GenPid(imgd.job), "imgresize", []string{t.fn, ThumbName(t.fn), strconv.Itoa(imgd.nrounds)})
	if imgd.crash > 0 {
		p.SetCrash(imgd.crash)
	}
	p.SetMcpu(imgd.workerMcpu)
	p.SetMem(imgd.workerMem)
	db.DPrintf(db.IMGD, "prep to spawn task %v %v", p.GetPid(), p.Args)
	start := time.Now()
	// Spawn proc.
	err := imgd.Spawn(p)
	if err != nil {
		db.DFatalf("Couldn't spawn a task %v, err: %v", t, err)
	}
	db.DPrintf(db.IMGD, "spawned task %v %v", p.GetPid(), p.Args)
	// Wait for results.
	res := imgd.waitForTask(start, p, t)
	if res.ok {
		db.DPrintf(db.IMGD, "%v ok %v ms %d msg %v", res.t, res.ok, res.ms, res.msg)
	}
	if !res.ok && res.msg != "" {
		db.DFatalf("task %v has unrecoverable err %v\n", res.t, res.msg)
	}
}

func (imgd *ImgSrv) work(sts []*sp.Stat) bool {
	// Due to inconsistent views of the WIP directory (concurrent adds by clients
	// and paging reads in the parent of this function), some entries may be
	// duplicated. Dedup them using this map.
	entries := make(map[string]bool)
	for _, st := range sts {
		entries[st.Name] = true
	}
	db.DPrintf(db.IMGD, "Removed %v duplicate entries", len(sts)-len(entries))
	for entry, _ := range entries {
		t, err := imgd.claimEntry(entry)
		if err != nil || t == "" {
			continue
		}
		s3fn, err := imgd.GetFile(path.Join(imgd.wip, t))
		if err != nil {
			continue
		}
		if string(s3fn) == STOP {
			return false
		}
		imgd.wg.Add(1)
		// Run the task in another thread.
		go imgd.runTask(&task{t, string(s3fn)})
	}
	db.DPrintf(db.IMGD, "Started %v tasks", len(entries))
	return true
}

// Consider all tasks in progress as failed (too aggressive, but
// correct), and make them runnable
func (imgd *ImgSrv) recover() {
	if _, err := imgd.MoveFiles(imgd.wip, imgd.todo); err != nil {
		db.DFatalf("MoveFiles %v err %v", imgd.wip, err)
	}
}

func (imgd *ImgSrv) Work() {

	db.DPrintf(db.IMGD, "Try acquire leadership coord %v job %v", imgd.ProcEnv().GetPID(), imgd.job)

	// Try to become the leading coordinator.
	if err := imgd.leaderclnt.LeadAndFence(nil, []string{path.Join(IMG, imgd.job)}); err != nil {
		db.DFatalf("LeadAndFence err %v", err)
	}

	db.DPrintf(db.ALWAYS, "leader %s", imgd.job)

	imgd.recover()

	work := true
	for work {
		db.DPrintf(db.IMGD, "ReadDirWatch %v", imgd.todo)
		sts, err := imgd.ReadDirWatch(imgd.todo, func(sts []*sp.Stat) bool {
			return len(sts) == 0
		})
		if err != nil {
			db.DFatalf("ReadDirWatch %v err %v", imgd.todo, err)
		}
		db.DPrintf(db.IMGD, "ReadDirWatch done %v, %v entries", imgd.todo, len(sts))
		work = imgd.work(sts)
	}
	imgd.wg.Wait()

	db.DPrintf(db.ALWAYS, "imgresized exit")

	imgd.exited = true
	imgd.ClntExitOK()
}
