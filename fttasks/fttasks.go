// The tasks package implements a bag of fault-tolerant tasks using named.
package fttasks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fslib"
	rd "sigmaos/rand"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
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
	return NewFtTasks(fsl, dir, job)
}

func NewFtTasks(fsl *fslib.FsLib, dir, job string) (*FtTasks, error) {
	ft := &FtTasks{FsLib: fsl, dir: dir, job: job}
	ft.done = filepath.Join(dir, job, "done")
	ft.todo = filepath.Join(dir, job, "todo")
	ft.wip = filepath.Join(dir, job, "wip")
	ft.error = filepath.Join(dir, job, "error")
	return ft, nil
}

func (ft *FtTasks) Cleanup() error {
	return ft.RmDir(filepath.Join(ft.dir, ft.job))
}

func (ft *FtTasks) Jobs() ([]*sp.Stat, error) {
	return ft.GetDir(ft.dir)
}

func (ft *FtTasks) NTasksTODO() (int, error) {
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

// Causes the server to stop after processing remaining tasks
func (ft *FtTasks) SubmitStop() error {
	db.DPrintf(db.FTTASKS, "SubmitStop")
	t := filepath.Join(ft.todo, STOP)
	_, err := ft.PutFile(t, 0777, sp.OWRITE, []byte{})
	return err
}

func (ft *FtTasks) SubmitTask(id int, i interface{}) error {
	db.DPrintf(db.FTTASKS, "SubmitTask id %v t %v", id, i)
	tid := strconv.Itoa(id) + "-" + rd.String(4)
	t := filepath.Join(ft.todo, tid)
	return ft.PutFileJson(t, 0777, i)
}

func (ft *FtTasks) SubmitTaskMulti(id int, is []interface{}) error {
	bs := make([]byte, 0)
	for _, i := range is {
		b, err := json.Marshal(i)
		if err != nil {
			return fmt.Errorf("Marshal error %v", err)
		}
		bs = append(bs, b...)
	}
	tid := strconv.Itoa(id) + "-" + rd.String(4)
	t := filepath.Join(ft.todo, tid)
	db.DPrintf(db.FTTASKS, "SubmitTaskMulti id %v tname %v", id, tid)
	_, err := ft.PutFile(t, 0777, sp.OWRITE, bs)
	return err
}

// Consider all tasks in progress as failed (too aggressive, but
// correct), and make them runnable.
func (ft *FtTasks) RecoverTasks() error {
	_, err := ft.MoveFiles(ft.wip, ft.todo)
	return err
}

func (ft *FtTasks) WaitForTasks() ([]string, error) {
	dr := fslib.NewDirReader(ft.FsLib, ft.todo)
	fns, err := dr.WatchNewEntriesAndRename(ft.wip)
	if err != nil {
		return nil, err
	}
	return fns, nil
}

func (ft *FtTasks) GetTasks() ([]string, error) {
	dr := fslib.NewDirReader(ft.FsLib, ft.todo)
	return dr.GetEntriesRename(ft.wip)
}

// Read tasks by reading file in one shot
func (ft *FtTasks) ReadTask(name string, i interface{}) error {
	return ft.GetFileJson(ft.wip+"/"+name, i)
}

// Read tasks using a reader
func (ft *FtTasks) TaskReader(name string) (*fslib.FdReader, error) {
	return ft.OpenReader(filepath.Join(ft.wip, name))
}

func (ft *FtTasks) TaskPathName(name string) string {
	return ft.wip + "/" + name
}

func (ft *FtTasks) MarkDone(name string) error {
	return ft.Rename(ft.wip+"/"+name, ft.done+"/"+name)
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
func (ft *FtTasks) Restart() (int, error) {
	return ft.MoveFiles(ft.error, ft.todo)
}
