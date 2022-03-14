package epochclnt

import (
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/rand"
)

type EpochClnt struct {
	*fslib.FsLib
	path    string
	perm    np.Tperm
	mode    np.Tmode
	f       *np.Tfence
	lastSeq np.Tseqno
	paths   map[string]bool
}

func MakeEpochClnt(fsl *fslib.FsLib, path string, perm np.Tperm, paths []string) *EpochClnt {
	ec := &EpochClnt{}
	ec.FsLib = fsl
	ec.path = path
	ec.perm = perm
	ec.paths = make(map[string]bool)
	for _, p := range paths {
		ec.paths[p] = true
	}
	return ec
}

func (ec *EpochClnt) Name() string {
	return ec.path
}

func (ec *EpochClnt) Fence() (np.Tfence, error) {
	if ec.f == nil {
		return np.Tfence{}, fmt.Errorf("Fence: not acquired %v\n", ec.path)
	}
	return *ec.f, nil
}

// XXX MakeFence should be on the first of the file written/read
func (ec *EpochClnt) FenceOffEpoch() error {
	fence, err := ec.MakeFence(ec.path, ec.mode)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "MakeFence %v err %v", ec.path, err)
		return err
	}
	if ec.lastSeq > fence.Seqno {
		log.Fatalf("%v: FATAL MakeFence bad fence %v last seq %v\n", proc.GetName(), fence, ec.lastSeq)
	}
	for p, _ := range ec.paths {
		err := ec.RegisterFence(fence, p)
		if err != nil {
			db.DLPrintf("EPOCHCLNT_ERR", "RegisterFence %v err %v", ec.path, err)
			return err
		}
	}
	ec.lastSeq = fence.Seqno
	ec.f = &fence
	return nil
}

//
// A epoch operations that increase the epoch's seqno, and
// registerEpoch will update servers to use the new epoch.
//
// XXX this relies on the leader file and epoch file are at the same
// server, so that after a network partition after becoming leader but
// before writing epoch file, the epoch write won't succeed.  Make
// conditional on the version of the leader file.
//

func (ec *EpochClnt) SetEpochFile(b []byte) error {
	_, err := ec.SetFile(ec.path, b, 0)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "SetEpochFile %v err %v", ec.path, err)
		return err
	}
	return ec.FenceOffEpoch()
}

func (ec *EpochClnt) MakeEpochFileFrom(from string) error {
	err := ec.Rename(from, ec.path)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "MakeEpochFileFrom %v to %v err %v", from, ec.path, err)
		return err
	}
	return ec.FenceOffEpoch()
}

func (ec *EpochClnt) MakeEpochFileJson(a interface{}) error {
	fn := ec.path + rand.String(16)
	err := ec.PutFileJson(fn, ec.perm, a)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "PutFile %v failed %v\n", fn, err)
	}
	err = ec.Rename(fn, ec.path)
	if err != nil {
		db.DLPrintf("EPOCHCLNT_ERR", "MakeEpochFileFrom %v to %v err %v", fn, ec.path, err)
		return err
	}
	return ec.FenceOffEpoch()
}

// Remove epoch.  The caller better sure there is no client relying on
// server checking this epoch.  The caller must have ended epoch.
// fence.
func (ec *EpochClnt) RemoveEpoch() error {
	if ec.f != nil {
		log.Fatalf("%v: FATAL RmFence %v\n", proc.GetName(), ec.path)
	}
	err := ec.RmFence(*ec.f, ec.path)
	if err != nil {
		return err
	}
	return nil
}
