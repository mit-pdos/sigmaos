package kv

import (
	"fmt"
	"path"
	"sync"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fenceclnt"
	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

// XXX cmd line utility cp

type Mover struct {
	mu sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	fclnt    *fenceclnt.FenceClnt
	job      string
	epochstr string
}

func JoinEpoch(fsl *fslib.FsLib, job, label, epochstr string, dirs []string) error {
	epoch, err := np.String2Epoch(epochstr)
	if err != nil {
		return err
	}
	fclnt := fenceclnt.MakeLeaderFenceClnt(fsl, KVBalancer(job))
	if err := fclnt.FenceAtEpoch(epoch, dirs); err != nil {
		return fmt.Errorf("FenceAtEpoch %v err %v", KVConfig(job), err)
	}
	// reads are fenced
	config := Config{}
	if err := fsl.GetFileJson(KVConfig(job), &config); err != nil {
		return fmt.Errorf("GetFileJson %v err %v", KVConfig(job), err)
	}
	if config.Epoch != epoch {
		return fmt.Errorf("Newer config %v", config.Epoch)
	}
	return nil
}

func MakeMover(job, epochstr, src, dst string) (*Mover, error) {
	mv := &Mover{}
	mv.epochstr = epochstr
	mv.FsLib = fslib.MakeFsLib("mover-" + proc.GetPid().String())
	mv.ProcClnt = procclnt.MakeProcClnt(mv.FsLib)
	mv.job = job

	if err := mv.Started(); err != nil {
		db.DFatalf("%v: couldn't start %v\n", proc.GetName(), err)
	}
	crash.Crasher(mv.FsLib)

	if err := JoinEpoch(mv.FsLib, mv.job, "KVMV", epochstr, []string{JobDir(mv.job), path.Dir(src), path.Dir(dst)}); err != nil {
		mv.Exited(proc.MakeStatusErr(err.Error(), nil))
		return nil, err
	}
	return mv, nil
}

func shardTmp(shardp string) string {
	return shardp + "#"
}

// Copy shard from src to dst
func (mv *Mover) copyShard(s, d string) error {
	d1 := shardTmp(d)

	// The previous mover might have crashed right after rename
	// below. If so, we are done.
	_, err := mv.Stat(d)
	if err == nil {
		db.DPrintf("KVMV_ERR", "moveShard conf %v exists %v\n", mv.epochstr, d)
		return nil
	}

	// An aborted view change may have created the directory and
	// partially copied files into it; remove it and start over.
	mv.RmDir(d1)

	err = mv.MkDir(d1, 0777)
	if err != nil {
		db.DPrintf("KVMV_ERR", "Mkdir %v err %v\n", d1, err)
		return err
	}
	// log.Printf("%v: Copy shard from %v to %v\n", proc.GetName(), s, d1)
	err = mv.CopyDir(s, d1)
	if err != nil {
		db.DPrintf("KVMV_ERR", "CopyDir shard%v to %v err %v\n", s, d1, err)
		return err
	}
	// log.Printf("%v: Copy shard%v to %v done\n", proc.GetName(), s, d1)
	err = mv.Rename(d1, d)
	if err != nil {
		db.DPrintf("KVMV_ERR", "Rename %v to %v err %v\n", d1, d, err)
		return err
	}
	return nil
}

func (mv *Mover) delShard(sharddir string) {
	db.DPrintf("KVMV", "conf %v delete %v\n", mv.epochstr, sharddir)

	// If sharddir isn't found, then an earlier delete succeeded;
	// we are done.
	if _, err := mv.Stat(sharddir); err != nil && np.IsErrNotfound(err) {
		db.DPrintf("KVMV_ERR", "Delete conf %v not found %v\n", mv.epochstr, sharddir)
		mv.Exited(proc.MakeStatus(proc.StatusOK))
		return
	}

	if err := mv.RmDir(sharddir); err != nil {
		db.DPrintf("KVMV_ERR", "conf %v rmdir %v err %v\n", mv.epochstr, sharddir, err)
		mv.Exited(proc.MakeStatusErr(err.Error(), nil))
	} else {
		mv.Exited(proc.MakeStatus(proc.StatusOK))
	}
}

func (mv *Mover) Move(src, dst string) {
	db.DPrintf("KVMV", "conf %v: mv from %v to %v\n", mv.epochstr, src, dst)
	err := mv.copyShard(src, dst)
	if err != nil {
		db.DPrintf("KVMV_ERR", "conf %v from %v to %v err %v\n", mv.epochstr, src, dst, err)
	}
	db.DPrintf("KVMV", "conf %v: mv done from %v to %v\n", mv.epochstr, src, dst)
	if err != nil {
		mv.Exited(proc.MakeStatusErr(err.Error(), nil))
	} else {
		mv.delShard(src)
	}
}
