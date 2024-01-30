// The tasks package implements a bag of fault-tolerant tasks using named.
package fttasks

import (
	"path"
	"strings"

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
	if err := fsl.MkDir(dir, 0777); err != nil {
		return nil, err
	}
	// job can be a pathname
	if err := fsl.MkDirPath(dir, job, 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(path.Join(dir, job, "done"), 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(path.Join(dir, job, "todo"), 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(path.Join(dir, job, "wip"), 0777); err != nil {
		return nil, err
	}
	if err := fsl.MkDir(path.Join(dir, job, "error"), 0777); err != nil {
		return nil, err
	}
	return NewFtTasks(fsl, dir, job)
}

func NewFtTasks(fsl *fslib.FsLib, dir, job string) (*FtTasks, error) {
	ft := &FtTasks{FsLib: fsl, dir: dir, job: job}
	ft.done = path.Join(dir, job, "done")
	ft.todo = path.Join(dir, job, "todo")
	ft.wip = path.Join(dir, job, "wip")
	ft.error = path.Join(dir, job, "error")
	return ft, nil
}

func (ft *FtTasks) Jobs() ([]*sp.Stat, error) {
	return ft.GetDir(ft.dir)
}

func (ft *FtTasks) NTaskDone() (int, error) {
	sts, err := ft.GetDir(ft.done)
	if err != nil {
		return -1, err
	}
	return len(sts), nil
}

func (ft *FtTasks) SubmitTask(fn string) error {
	return ft.SubmitTaskMulti([]string{fn})
}

func (ft *FtTasks) SubmitTaskMulti(fns []string) error {
	db.DPrintf(db.FTTASKS, "SubmitTaskMulti %v", fns)
	t := path.Join(sp.IMG, ft.job, "todo", rd.String(4))
	_, err := ft.PutFile(t, 0777, sp.OREAD, []byte(strings.Join(fns, ",")))
	return err
}

// Consider all tasks in progress as failed (too aggressive, but
// correct), and make them runnable.
func (ft *FtTasks) RecoverTasks() error {
	_, err := ft.MoveFiles(ft.wip, ft.todo)
	return err
}

func (ft *FtTasks) WaitForTasks() ([]*sp.Stat, error) {
	db.DPrintf(db.FTTASKS, "ReadDirWatch %v", ft.todo)
	var sts []*sp.Stat
	err := ft.ReadDirWait(ft.todo, func(sts0 []*sp.Stat) bool {
		sts = sts0
		return len(sts0) == 0
	})
	db.DPrintf(db.FTTASKS, "ReadDirWatch done %v, %v entries", ft.todo, len(sts))
	return sts, err
}

func (ft *FtTasks) ClaimTask(name string) (string, error) {
	if err := ft.Rename(ft.todo+"/"+name, ft.wip+"/"+name); err != nil {
		if serr.IsErrCode(err, serr.TErrUnreachable) { // partitioned?
			return "", err
		}
		// another thread claimed the task before us
		db.DPrintf(db.FTTASKS, "Error claim entry %v: %v", name, err)
		return "", nil
	}
	db.DPrintf(db.FTTASKS, "Claim %v success", name)
	return name, nil
}

func (ft *FtTasks) ReadTask(name string) ([]byte, error) {
	return ft.GetFile(path.Join(ft.wip, name))
}

func (ft *FtTasks) MarkDone(name string) error {
	return ft.Rename(ft.wip+"/"+name, ft.done+"/"+name)
}

func (ft *FtTasks) MarkRunnable(name string) error {
	return ft.Rename(ft.wip+"/"+name, ft.todo+"/"+name)
}
