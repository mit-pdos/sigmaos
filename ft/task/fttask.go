// The tasks package implements a bag of fault-tolerant tasks using named.
package task

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sigmaclnt/fslib/dirwatcher"
	sp "sigmaos/sigmap"
	protsrv_proto "sigmaos/spproto/srv/proto"
	rd "sigmaos/util/rand"
)

const (
	STOP = "_STOP"
)

type FtTasks struct {
	*fslib.FsLib
	dir   string
	job   string
	done  string
	wip   string
	todo  string
	error string
	stats string
	sts   Stats
}

type Stats struct {
	Ntask int
}

func MkFtTasks(fsl *fslib.FsLib, dir, job string) (*FtTasks, error) {
	if err := fsl.MkDir(dir, 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		return nil, err
	}
	// job can be a pathname
	if err := fsl.MkDirPath(dir, job, 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(filepath.Join(dir, job, "done"), 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(filepath.Join(dir, job, "todo"), 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(filepath.Join(dir, job, "wip"), 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(filepath.Join(dir, job, "error"), 0777); err != nil {
		return nil, err
	}
	sts := Stats{}
	if err := fsl.PutFileJson(filepath.Join(dir, job, "stats"), 0777, sts); err != nil {
		return nil, err
	}
	return NewFtTasks(fsl, dir, job)
}

func NewFtTasks(fsl *fslib.FsLib, dir, job string) (*FtTasks, error) {
	ft := &FtTasks{FsLib: fsl, dir: dir, job: job}
	ft.done = filepath.Join(dir, job, "done")
	ft.todo = filepath.Join(dir, job, "todo")
	ft.wip = filepath.Join(dir, job, "wip")
	ft.error = filepath.Join(dir, job, "error")
	ft.stats = filepath.Join(dir, job, "stats")
	if err := fsl.GetFileJson(ft.stats, &ft.sts); err != nil {
		return nil, err
	}
	return ft, nil
}

func (ft *FtTasks) Cleanup() error {
	return ft.RmDir(filepath.Join(ft.dir, ft.job))
}

func (ft *FtTasks) Jobs() ([]*sp.Tstat, error) {
	return ft.GetDir(ft.dir)
}

func (ft *FtTasks) JobState() (string, error) {
	return ft.SprintfDir(filepath.Join(ft.dir, ft.job))
}

func (ft *FtTasks) NTasksToDo() (int, error) {
	sts, err := ft.GetDir(ft.todo)
	if err != nil {
		return -1, err
	}
	return len(sts), nil
}

func (ft *FtTasks) NTaskDone() (int, error) {
	sts, err := ft.GetDir(ft.done)
	if err != nil {
		return -1, err
	}
	return len(sts), nil
}

func (ft *FtTasks) updateStats(n int) error {
	ft.sts.Ntask += n
	if err := ft.SetFileJson(ft.stats, ft.sts); err != nil {
		return err
	}
	return nil
}

func (ft *FtTasks) GetStats() Stats {
	sts := ft.sts
	return sts
}

func (ft *FtTasks) getTasks(dir string) ([]string, error) {
	sts, err := ft.GetDir(dir)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}

func (ft *FtTasks) GetDoneTasks() ([]string, error) {
	return ft.getTasks(ft.done)
}

func (ft *FtTasks) GetTodoTasks() ([]string, error) {
	return ft.getTasks(ft.todo)
}

// Causes the server to stop after processing remaining tasks
func (ft *FtTasks) SubmitStop() error {
	db.DPrintf(db.FTTASKS, "SubmitStop")
	t := filepath.Join(ft.todo, STOP)
	_, err := ft.PutFile(t, 0777, sp.OWRITE, []byte{})
	return err
}

func (ft *FtTasks) mkTask(tid string) error {
	t := filepath.Join(ft.todo, tid)
	if err := ft.MkDir(t, 0777); err != nil {
		return err
	}
	return nil
}

func (ft *FtTasks) SubmitTask(id int, i any) error {
	db.DPrintf(db.FTTASKS, "SubmitTask id %v t %v", id, i)
	tid := strconv.Itoa(id) + "-" + rd.String(16)
	if err := ft.mkTask(tid); err != nil {
		return err
	}
	t := filepath.Join(ft.todo, tid, "task")
	if err := ft.PutFileJson(t, 0777, i); err != nil {
		return err
	}
	if err := ft.updateStats(1); err != nil {
		return err
	}
	return nil
}

func (ft *FtTasks) SubmitTaskMulti(id int, is []any) error {
	bs := make([]byte, 0)
	for _, i := range is {
		b, err := json.Marshal(i)
		if err != nil {
			return fmt.Errorf("Marshal error %v", err)
		}
		bs = append(bs, b...)
	}
	tid := strconv.Itoa(id) + "-" + rd.String(4)
	if err := ft.mkTask(tid); err != nil {
		return err
	}
	t := filepath.Join(ft.todo, tid, "task")
	db.DPrintf(db.FTTASKS, "SubmitTaskMulti id %v tname %v", id, tid)
	_, err := ft.PutFile(t, 0777, sp.OWRITE, bs)

	if err := ft.updateStats(len(bs)); err != nil {
		return err
	}

	return err
}

// Consider all tasks in progress as failed (too aggressive, but
// correct), and make them runnable.
func (ft *FtTasks) RecoverTasks() (int, error) {
	n, err := ft.MoveDirEntries(ft.wip, ft.todo)
	if err != nil {
		return 0, err
	}
	if err := ft.updateStats(n); err != nil {
		return 0, err
	}
	return n, err
}

func (ft *FtTasks) renameTodoToWip(files []string) ([]string, error) {
	var r error
	newents := make([]string, 0)

	for _, file := range files {
		if err := ft.Rename(filepath.Join(ft.todo, file), filepath.Join(ft.wip, file)); err == nil {
			newents = append(newents, file)
		} else if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
			r = err
			break
		}
	}
	return newents, r
}

func (ft *FtTasks) WaitForTasks() ([]string, error) {
	ents, dw, err := dirwatcher.NewDirWatcherWithRead(ft.FsLib, ft.todo)
	defer dw.Close()

	if err != nil {
		return nil, err
	}

	for {
		renamed, err := ft.renameTodoToWip(ents)
		if err != nil {
			return nil, err
		}
		if len(renamed) > 0 {
			return renamed, nil
		}

		for event := range dw.Events() {
			if event.Type == protsrv_proto.WatchEventType_CREATE {
				ents = []string{event.File}
				break
			}
		}
	}
}

func (ft *FtTasks) AcquireTasks() ([]string, error) {
	ents := make([]string, 0)
	sts, err := ft.GetDir(ft.todo)
	if err != nil {
		return nil, err
	}
	for _, ent := range sts {
		ents = append(ents, ent.Name)
	}
	return ft.renameTodoToWip(ents)
}

// Read tasks by reading file in one shot
func (ft *FtTasks) ReadTask(name string, i any) error {
	return ft.GetFileJson(filepath.Join(ft.wip, name, "task"), i)
}

// Read tasks's output
func (ft *FtTasks) ReadTaskOutput(name string, i any) error {
	return ft.GetFileJson(filepath.Join(ft.done, name, "output"), i)
}

// Read tasks using a reader
func (ft *FtTasks) TaskReader(name string) (*fslib.FileReader, error) {
	rdr, err := ft.OpenReader(filepath.Join(ft.wip, name, "task"))
	if err != nil {
		return nil, err
	}
	return rdr, nil
}

func (ft *FtTasks) TaskPathName(name string) string {
	return filepath.Join(ft.wip, name, "task")
}

func (ft *FtTasks) MarkDone(name string) error {
	return ft.Rename(ft.wip+"/"+name, ft.done+"/"+name)
}

func (ft *FtTasks) MarkDoneOutput(name string, i any) error {
	t := filepath.Join(ft.wip, name, "output")
	if err := ft.PutFileJson(t, 0777, i); err != nil {
		return err
	}
	return ft.MarkDone(name)
}

func (ft *FtTasks) MarkRunnable(name string) error {
	return ft.Rename(ft.wip+"/"+name, ft.todo+"/"+name)
}

// Mark a task as errored out to restart it later
func (ft *FtTasks) MarkError(name string) error {
	if err := ft.Rename(ft.done+"/"+name, ft.error+"/"+name); err == nil {
		return nil
	}
	return ft.Rename(ft.wip+"/"+name, ft.error+"/"+name)
}

// Mark all error-ed tasks as runnable
func (ft *FtTasks) MarkErrorTodo() (int, error) {
	n, err := ft.MoveDirEntries(ft.error, ft.todo)
	if err != nil {
		return 0, err
	}
	if err := ft.updateStats(n); err != nil {
		return 0, err
	}
	return n, err
}

// Mark all error-ed tasks as runnable
func (ft *FtTasks) MarkDoneError() (int, error) {
	return ft.MoveDirEntries(ft.done, ft.error)
}
