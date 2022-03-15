package fenceclnt1

import (
	db "ulambda/debug"
	"ulambda/epochclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type FenceClnt struct {
	*fslib.FsLib
	ec      *epochclnt.EpochClnt
	perm    np.Tperm
	mode    np.Tmode
	f       *np.Tfence
	lastSeq np.Tseqno
	paths   map[string]bool
}

func MakeFenceClnt(fsl *fslib.FsLib, ec *epochclnt.EpochClnt, perm np.Tperm, paths []string) *FenceClnt {
	fc := &FenceClnt{}
	fc.FsLib = fsl
	fc.ec = ec
	fc.perm = perm
	fc.paths = make(map[string]bool)
	for _, p := range paths {
		fc.paths[p] = true
	}
	return fc
}

func (fc *FenceClnt) FenceAtEpoch(epoch string) error {
	f, err := fc.ec.GetFence(epoch)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "GetFence %v err %v", fc.ec.Name(), err)
	}

	fc.fencePaths(&f)
	return nil
}

func (fc *FenceClnt) fencePaths(fence *np.Tfence1) error {
	for p, _ := range fc.paths {
		err := fc.registerFence(p, fence)
		if err != nil {
			db.DLPrintf("FENCECLNT_ERR", "fencePath %v err %v", p, err)
			return err
		}
		fc.paths[p] = true
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
