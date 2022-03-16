package fenceclnt1

import (
	db "ulambda/debug"
	"ulambda/epochclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type FenceClnt struct {
	*fslib.FsLib
	*epochclnt.EpochClnt
	perm    np.Tperm
	mode    np.Tmode
	f       *np.Tfence
	lastSeq np.Tseqno
	paths   map[string]bool
}

func MakeFenceClnt(fsl *fslib.FsLib, ec *epochclnt.EpochClnt) *FenceClnt {
	fc := &FenceClnt{}
	fc.FsLib = fsl
	fc.EpochClnt = ec
	return fc
}

func MakeLeaderFenceClnt(fsl *fslib.FsLib, leaderfn string) *FenceClnt {
	fc := &FenceClnt{}
	fc.FsLib = fsl
	fc.EpochClnt = epochclnt.MakeEpochClnt(fsl, leaderfn, 0777)
	return fc
}

func (fc *FenceClnt) FenceAtEpoch(epoch np.Tepoch, paths []string) error {
	f, err := fc.GetFence(epoch)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "GetFence %v err %v", fc.Name(), err)
	}
	db.DLPrintf("FENCECLNT", "FenceAtEpoch %v %v", epoch, paths)
	return fc.fencePaths(&f, paths)
}

func (fc *FenceClnt) ReadEpoch() (np.Tepoch, error) {
	return fc.GetEpoch()
}

func (fc *FenceClnt) fencePaths(fence *np.Tfence1, paths []string) error {
	for _, p := range paths {
		err := fc.registerFence(p, fence)
		if err != nil {
			db.DLPrintf("FENCECLNT_ERR", "fencePath %v err %v", p, err)
			return err
		}
	}
	return nil
}

func (fc *FenceClnt) registerFence(path string, fence *np.Tfence1) error {
	// maybe stat to register fence?
	if err := fc.FenceDir(path, fence); err != nil {
		return err
	}
	return nil
}
