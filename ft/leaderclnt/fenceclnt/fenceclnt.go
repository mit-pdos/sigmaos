package fenceclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
)

type FenceClnt struct {
	*fslib.FsLib
	fence sp.Tfence
}

func NewFenceClnt(fsl *fslib.FsLib) *FenceClnt {
	fc := &FenceClnt{}
	fc.FsLib = fsl
	return fc
}

// Future operations on files in a tree rooted at a path in paths will
// be fenced by <fence>
func (fc *FenceClnt) FenceAtEpoch(fence sp.Tfence, paths []string) error {
	db.DPrintf(db.FENCECLNT, "FencePaths fence %v %v", fence, paths)
	fc.fence = fence
	for _, p := range paths {
		if err := fc.registerFence(p, fence); err != nil {
			db.DPrintf(db.FENCECLNT_ERR, "fencePath %v err %v", p, err)
			return err
		}
	}
	return nil
}

// Register fence with fidclnt so that ops on files in the tree rooted
// at path will include fence.
func (fc *FenceClnt) registerFence(path string, fence sp.Tfence) error {
	if err := fc.FenceDir(path, fence); err != nil {
		return err
	}
	// Inform servers of fence with new epoch, but unnecessary?
	// The next op to path (or child) will include the new fence
	// but now servers will learn about new epoch at the different
	// times.
	//
	//if _, err := fc.GetDir(path + "/"); err != nil {
	//	db.DPrintf(db.FENCECLNT_ERR, "WARNING getdir %v err %v\n", path, err)
	//	return err
	//}
	return nil
}

func (fc *FenceClnt) GetFences(p string) ([]*sp.Tstat, error) {
	srv, _, err := fc.PathLastMount(p)
	if err != nil {
		db.DPrintf(db.FENCECLNT_ERR, "PathLastMount %v err %v", p, err)
		return nil, err
	}
	dn := srv.String() + "/" + sp.FENCEDIR
	sts, err := fc.GetDir(dn)
	if err != nil {
		db.DPrintf(db.FENCECLNT_ERR, "GetDir %v err %v", dn, err)
	}
	return sts, nil
}

func (fc *FenceClnt) RemoveFence(dirs []string) error {
	for _, d := range dirs {
		srv, _, err := fc.PathLastMount(d)
		if err != nil {
			db.DPrintf(db.FENCECLNT_ERR, "PathLastMount %v err %v", d, err)
			return err
		}
		fn := srv.String() + "/" + sp.FENCEDIR + "/" + fc.fence.Name()
		if err := fc.Remove(fn); err != nil {
			db.DPrintf(db.FENCECLNT_ERR, "Remove %v err %v", fn, err)
			return err
		}
	}
	return nil
}
