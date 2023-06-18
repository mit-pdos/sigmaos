package fenceclnt

import (
	db "sigmaos/debug"
	"sigmaos/epochclnt"
	"sigmaos/fslib"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type FenceClnt struct {
	*fslib.FsLib
	*epochclnt.EpochClnt
	perm    sp.Tperm
	mode    sp.Tmode
	lastSeq sessp.Tseqno
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
func (fc *FenceClnt) FenceAtEpoch(epoch sessp.Tepoch, paths []string) error {
	f, err := fc.GetFence(epoch)
	if err != nil {
		db.DPrintf(db.FENCECLNT_ERR, "GetFence %v err %v", fc.Name(), err)
		return err
	}
	return fc.fencePaths(f, paths)
}

func (fc *FenceClnt) fencePaths(fence *sessp.Tfence, paths []string) error {
	db.DPrintf(db.FENCECLNT, "FencePaths fence %v %v", fence, paths)
	for _, p := range paths {
		err := fc.registerFence(p, *fence)
		if err != nil {
			db.DPrintf(db.FENCECLNT_ERR, "fencePath %v err %v", p, err)
			return err
		}
	}
	return nil
}

// Register fence with fidclnt so that ops on files in the tree rooted
// at path will include fence.
func (fc *FenceClnt) registerFence(path string, fence sessp.Tfence) error {
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

func (fc *FenceClnt) GetFences(p string) ([]*sp.Stat, error) {
	srv, _, err := fc.PathLastSymlink(p)
	if err != nil {
		db.DPrintf(db.FENCECLNT_ERR, "PathLastSymlink %v err %v", p, err)
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
	e, err := fc.ReadEpoch()
	if err != nil {
		db.DPrintf(db.FENCECLNT_ERR, "ReadEpoch %v err %v", fc.Name(), err)
		return err
	}
	f, err := fc.GetFence(e)
	if err != nil {
		db.DPrintf(db.FENCECLNT_ERR, "GetFence %v err %v", fc.Name(), err)
		return err
	}
	for _, d := range dirs {
		srv, _, err := fc.PathLastSymlink(d)
		if err != nil {
			db.DPrintf(db.FENCECLNT_ERR, "PathLastSymlink %v err %v", d, err)
			return err
		}
		fn := srv.String() + "/" + sp.FENCEDIR + "/" + f.Fenceid.Tpath().String()
		if err := fc.Remove(fn); err != nil {
			db.DPrintf(db.FENCECLNT_ERR, "Remove %v err %v", fn, err)
			return err
		}
	}
	return nil
}
