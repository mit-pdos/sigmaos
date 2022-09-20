package fsux

import (
	"os"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/file"
	"sigmaos/fs"
	np "sigmaos/ninep"
)

type Symlink struct {
	*Obj
	*file.File
}

func makeSymlink(path np.Path, iscreate bool) (*Symlink, *np.Err) {
	s := &Symlink{}
	o, err := makeObj(path)
	if err == nil && iscreate {
		return nil, np.MkErr(np.TErrExists, path)
	}
	s.Obj = o
	s.File = file.MakeFile()
	return s, nil
}

func (s *Symlink) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("UXD", "%v: SymOpen %v m %x\n", ctx, s, m)
	if m&np.OWRITE == np.OWRITE {
		// no calls to update target of an existing symlink,
		// so remove it.  close() will make the symlink with
		// the new target.
		os.Remove(s.Obj.pathName.String())
	}
	if m&0x1 == np.OREAD {
		// read the target and write it to the in-memory file,
		// so that Read() can read it.
		target, error := os.Readlink(s.Obj.pathName.String())
		if error != nil {
			return nil, UxTo9PError(error, s.Obj.pathName.Base())
		}
		db.DPrintf("UXD", "Readlink target='%s'\n", target)
		d := []byte(target)
		_, err := s.File.Write(ctx, 0, d, np.NoV)
		if err != nil {
			db.DPrintf("UXD", "Write %v err %v\n", s, err)
			return nil, err
		}
	}
	return nil, nil
}

func (s *Symlink) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	db.DPrintf("UXD", "%v: SymClose %v %x\n", ctx, s, mode)
	if mode&np.OWRITE == np.OWRITE {
		d, err := s.File.Read(ctx, 0, np.MAXGETSET, np.NoV)
		if err != nil {
			return err
		}
		error := syscall.Symlink(string(d), s.Obj.pathName.String())
		if error != nil {
			db.DPrintf("UXD", "symlink %s err %v\n", s, error)
			UxTo9PError(error, s.Obj.pathName.Base())
		}
	}
	return nil
}
