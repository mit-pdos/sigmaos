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

// Future operations on files in a tree rooted at a path in paths will
// include a fence at epoch <epoch>.
func (fc *FenceClnt) FenceAtEpoch(epoch np.Tepoch, paths []string) error {
	f, err := fc.GetFence(epoch)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "GetFence %v err %v", fc.Name(), err)
		return err
	}
	return fc.fencePaths(f, paths)
}

// Unfence paths
func (fc *FenceClnt) Unfence(paths []string) error {
	return fc.unfencePaths(paths)
}

func (fc *FenceClnt) ReadEpoch() (np.Tepoch, error) {
	return fc.GetEpoch()
}

func (fc *FenceClnt) fencePaths(fence np.Tfence1, paths []string) error {
	db.DLPrintf("FENCECLNT", "FencePaths fence %v %v", fence, paths)
	for _, p := range paths {
		err := fc.registerFence(p, fence)
		if err != nil {
			db.DLPrintf("FENCECLNT_ERR", "fencePath %v err %v", p, err)
			return err
		}
	}
	return nil
}

func (fc *FenceClnt) unfencePaths(paths []string) error {
	db.DLPrintf("FENCECLNT", "UnfencePaths %v", paths)
	for _, p := range paths {
		err := fc.UnfenceDir(p)
		if err != nil {
			db.DLPrintf("FENCECLNT_ERR", "unfencePath %v err %v", p, err)
			return err
		}
	}
	return nil
}

// Register fence with fidclnt so that ops on files in the tree rooted
// at path will include fence.
func (fc *FenceClnt) registerFence(path string, fence np.Tfence1) error {
	if err := fc.FenceDir(path, fence); err != nil {
		return err
	}
	// Inform servers of fence with new epoch, but unnecessary?
	// The next op to path (or child) will include the new fence
	// but now servers will learn about new epoch at the different
	// times.
	//
	//if _, err := fc.GetDir(path + "/"); err != nil {
	//	db.DLPrintf("FENCECLNT_ERR", "WARNING getdir %v err %v\n", path, err)
	//	return err
	//}
	return nil
}
