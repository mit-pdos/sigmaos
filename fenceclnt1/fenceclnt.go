package fenceclnt1

import (
	"fmt"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type FenceClnt struct {
	*fslib.FsLib
	epochfn string // path for epoch file
	perm    np.Tperm
	mode    np.Tmode
	f       *np.Tfence
	lastSeq np.Tseqno
	paths   map[string]bool
}

func MakeFenceClnt(fsl *fslib.FsLib, epochfn string, perm np.Tperm, paths []string) *FenceClnt {
	fc := &FenceClnt{}
	fc.FsLib = fsl
	fc.epochfn = epochfn
	fc.perm = perm
	fc.paths = make(map[string]bool)
	for _, p := range paths {
		fc.paths[p] = true
	}
	return fc
}

func (fc *FenceClnt) Name() string {
	return fc.epochfn
}

func (fc *FenceClnt) FenceAtEpoch(epoch string) error {
	fd, err := fc.Open(fc.epochfn, np.OWRITE)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "Open %v err %v", fc.epochfn, err)
		return err
	}
	b, err := fc.Read(fd, 100)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "Read %v err %v", fc.epochfn, err)
		return err
	}
	if string(b) != epoch {
		db.DLPrintf("FENCECLNT_ERR", "Epoch mismatch %v err %v", fc.epochfn, err)
		return fmt.Errorf("Epoch mismatch %v %v\n", string(b), epoch)
	}
	qid, err := fc.Qid(fd)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "FdQid %v err %v", fd, err)
		return err
	}
	e, err := np.String2Epoch(epoch)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "String2Epoch %v err %v", epoch, err)
		return err
	}
	fc.fencePaths(np.MakeFence1(np.Tfenceid1{qid.Path, 0}, e))
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
	server, err := fc.PathServer(path)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "PathSerer %v err %v", path, err)
		return err
	}
	fencedir := server + "/" + np.FENCEDIR
	if err := fc.MkDir(fencedir, fc.perm); err != nil && !np.IsErrExists(err) {
		db.DLPrintf("FENCECLNT_ERR", "MkDir %v err %v", fencedir, err)
		return err
	}
	if err := fc.updateLatestEpoch(fencedir, fence); err != nil {
		return err
	}
	if err := fc.FenceDir(path, fence); err != nil {
		return err
	}
	return nil
}

// should use WriteV
func (fc *FenceClnt) updateLatestEpoch(fencedir string, f *np.Tfence1) error {
	fn := fencedir + f.FenceId.Path.String() // XXX should include server
	fd, err := fc.CreateOpen(fn, fc.perm, np.ORDWR)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "fence %v err %v", fn, err)
		return err
	}
	defer fc.Close(fd)
	b, err := fc.Read(fd, 100)
	if err != nil {
		db.DLPrintf("FENCECLNT_ERR", "Read %v err %v", fn, err)
		return err
	}
	lastepoch := np.Tepoch(0)
	if len(b) > 0 {
		lastepoch, err = np.String2Epoch(string(b))
		if err != nil {
			db.DLPrintf("EPOCHCLNT_ERR", "UnmarshalQid %v err %v", string(b), err)
			return err
		}
	}
	if f.Epoch < lastepoch {
		db.DLPrintf("EPOCHCLNT_ERR", "Stale fence %v lastest %v\n", f, lastepoch)
		return np.MkErr(np.TErrStale, f.FenceId)
	}
	if f.Epoch > lastepoch {
		db.DLPrintf("EPOCHCLNT", "Update fence's epoch %v lastest %v\n", f, lastepoch)
		err = fc.Seek(fd, 0)
		_, err = fc.Write(fd, []byte(f.Epoch.String()))
		if err != nil {
			db.DLPrintf("FENCECLNT_ERR", "Write %v err %v", fn, err)
			return err
		}
	}
	return nil
}
