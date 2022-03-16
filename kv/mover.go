package kv

import (
	"log"
	"path"
	"sync"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fenceclnt1"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

// XXX cmd line utility cp

type Mover struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	fclnt    *fenceclnt1.FenceClnt
	epochstr string
}

func JoinEpoch(fsl *fslib.FsLib, epochstr string, dirs []string) error {
	epoch, err := np.String2Epoch(epochstr)
	if err != nil {
		return err
	}
	fclnt := fenceclnt1.MakeLeaderFenceClnt(fsl, KVBALANCER)
	if err := fclnt.FenceAtEpoch(epoch, dirs); err != nil {
		return err
	}
	// reads are fenced
	config := Config{}
	if err := fsl.GetFileJson(KVCONFIG, &config); err != nil {
		db.DLPrintf("KVMV_ERR", "GetFileJson %v err %v\n", KVCONFIG, err)
		return err
	}
	if config.Epoch != epoch {
		return err
	}
	return nil
}

func MakeMover(epochstr, src, dst string) (*Mover, error) {
	mv := &Mover{}
	mv.epochstr = epochstr
	mv.FsLib = fslib.MakeFsLib("mover-" + proc.GetPid())
	mv.ProcClnt = procclnt.MakeProcClnt(mv.FsLib)

	if err := mv.Started(proc.GetPid()); err != nil {
		log.Fatalf("%v: couldn't start %v\n", proc.GetName(), err)
	}
	crash.Crasher(mv.FsLib)

	if err := JoinEpoch(mv.FsLib, epochstr, []string{KVDIR, src, path.Dir(dst)}); err != nil {
		mv.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
		return nil, err
	}

	return mv, nil
}

func shardTmp(shardp string) string {
	return shardp + "#"
}

// Move shard from src to dst
func (mv *Mover) moveShard(s, d string) error {
	d1 := shardTmp(d)

	// The previous mover might have crashed right after rename
	// below. If so, we are done.
	_, err := mv.Stat(d)
	if err == nil {
		db.DLPrintf("KVMV_ERR", "moveShard conf %v exists %v\n", mv.epochstr, d)
		return nil
	}

	// An aborted view change may have created the directory and
	// partially copied files into it; remove it and start over.
	mv.RmDir(d1)

	err = mv.MkDir(d1, 0777)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "Mkdir %v err %v\n", d1, err)
		return err
	}
	// log.Printf("%v: Copy shard from %v to %v\n", proc.GetName(), s, d1)
	err = mv.CopyDir(s, d1)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "CopyDir shard%v to %v err %v\n", s, d1, err)
		return err
	}
	// log.Printf("%v: Copy shard%v to %v done\n", proc.GetName(), s, d1)
	err = mv.Rename(d1, d)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "Rename %v to %v err %v\n", d1, d, err)
		return err
	}
	return nil
}

func (mv *Mover) Move(src, dst string) {
	db.DLPrintf("KVMV", "conf %v: mv from %v to %v\n", mv.epochstr, src, dst)
	err := mv.moveShard(src, dst)
	if err != nil {
		db.DLPrintf("KVMV_ERR", "conf %v from %v to %v err %v\n", mv.epochstr, src, dst, err)
	}
	if err != nil {
		mv.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
	} else {
		mv.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
	}
}
