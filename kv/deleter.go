package kv

import (
	"log"
	"sync"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

// XXX cmd line utility rmdir

type Deleter struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MakeDeleter() (*Deleter, error) {
	mv := &Deleter{}
	mv.FsLib = fslib.MakeFsLib("deleter-" + proc.GetPid())
	mv.ProcClnt = procclnt.MakeProcClnt(mv.FsLib)
	crash.Crasher(mv.FsLib)
	mv.Started(proc.GetPid())
	return mv, nil
}

func (dl *Deleter) Delete(sharddir string) {
	err := dl.RmDir(sharddir)
	if err != nil {
		log.Printf("%v: rmdir %v err %v\n", db.GetName(), sharddir, err)
		dl.Exited(proc.GetPid(), err.Error())
	} else {
		dl.Exited(proc.GetPid(), "OK")
	}
}
