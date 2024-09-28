package fslib

import (
	"fmt"
	"path/filepath"
	"time"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/reader"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (fl *FsLib) MkDir(path string, perm sp.Tperm) error {
	perm = perm | sp.DMDIR
	start := time.Now()
	fd, err := fl.Create(path, perm, sp.OREAD)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSLIB, "MkDir Create [%v]: %v", path, time.Since(start))
	start = time.Now()
	if err := fl.CloseFd(fd); err != nil {
		return err
	}
	db.DPrintf(db.FSLIB, "MkDir Close [%v]: %v", path, time.Since(start))
	return nil
}

func (fl *FsLib) IsDir(name string) (bool, error) {
	st, err := fl.FileAPI.Stat(name)
	if err != nil {
		return false, err
	}
	return st.Tmode().IsDir(), nil
}

// Create all parent dirs in pn if they don't exist.  If the last one
// exists, return error.
func (fl *FsLib) MkDirPath(dir, pn string, perm sp.Tperm) error {
	p := path.Split(pn)
	for i, c := range p {
		dir = filepath.Join(dir, c)
		err := fl.MkDir(dir, perm)
		if err == nil {
			continue
		}
		if !serr.IsErrCode(err, serr.TErrExists) {
			return err
		}
		if i == len(p)-1 {
			return err
		}
	}
	return nil
}

// ProcessDir pages through directories entries, calling f for each
// one them. Too stop early, f must return true.  ProcessDir returns
// true if stopped early.  Note that as ProcessDir pages through dir
// other procs may add/delete entries, which may cause ProcessDir to
// see the same file twice.  Use DirReader to filter duplicates.
func (fl *FsLib) ProcessDir(dir string, f func(*sp.Stat) (bool, error)) (bool, error) {
	rdr, err := fl.OpenReader(dir)
	if err != nil {
		return false, err
	}
	defer rdr.Close()
	return reader.ReadDirEnts(reader.MkDirEntsReader(rdr.(*FdReader).Reader), f)
}

func (fl *FsLib) GetDir(dir string) ([]*sp.Stat, error) {
	st, rdr, err := fl.ReadDir(dir)
	if rdr != nil {
		rdr.Close()
	}
	return st, err
}

// Also returns reader.Reader for readDirWatch
func (fl *FsLib) ReadDir(dir string) ([]*sp.Stat, *FdReader, error) {
	rdr, err := fl.OpenReader(dir)
	if err != nil {
		db.DPrintf(db.FSLIB_ERR, "Err ReadDir.OpenReader: %v", err)
		return nil, nil, err
	}
	dirents := []*sp.Stat{}
	_, error := reader.ReadDirEnts(reader.MkDirEntsReader(rdr.(*FdReader).Reader), func(st *sp.Stat) (bool, error) {
		dirents = append(dirents, st)
		return false, nil
	})
	if error != nil {
		db.DPrintf(db.FSLIB_ERR, "Err reader.ReadDir: %v", error)
	}
	return dirents, rdr.(*FdReader), error
}

// XXX should use Reader
func (fl *FsLib) CopyDir(src, dst string) error {
	_, err := fl.ProcessDir(src, func(st *sp.Stat) (bool, error) {
		s := filepath.Join(src, st.Name)
		d := filepath.Join(dst, st.Name)
		db.DPrintf(db.FSLIB, "CopyFile: %v %v\n", s, d)
		b, err := fl.GetFile(s)
		if err != nil {
			return true, err
		}
		_, err = fl.PutFile(d, 0777, sp.OWRITE, b)
		if err != nil {
			return true, err
		}
		return false, nil
	})
	return err
}

func (fl *FsLib) MoveDirEntries(src, dst string) (int, error) {
	sts, err := fl.GetDir(src) // XXX handle one entry at the time?
	if err != nil {
		return 0, err
	}
	n := 0
	for _, st := range sts {
		db.DPrintf(db.FSLIB, "move %v to %v\n", st.Name, dst)
		to := filepath.Join(dst, st.Name)
		if fl.Rename(filepath.Join(src, st.Name), to) != nil {
			return n, err
		}
		n += 1
	}
	return n, nil
}

func (fsl *FsLib) RmDir(dir string) error {
	if err := fsl.RmDirEntries(dir); err != nil {
		return err
	}
	return fsl.Remove(dir)
}

func (fsl *FsLib) RmDirEntries(dir string) error {
	sts, err := fsl.GetDir(dir)
	if err != nil {
		return err
	}
	for _, st := range sts {
		if st.Tmode().IsDir() {
			if err := fsl.RmDir(filepath.Join(dir, st.Name)); err != nil {
				return err
			}
		} else {
			if err := fsl.Remove(filepath.Join(dir, st.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (fsl *FsLib) SprintfDir(d string) (string, error) {
	s := fmt.Sprintf("dir %v:\n", d)
	s1, err := fsl.sprintfDirIndent(d, "")
	if err != nil {
		return "", err
	}
	return s + s1, nil
}

func (fsl *FsLib) sprintfDirIndent(d string, indent string) (string, error) {
	s := ""
	sts, err := fsl.GetDir(d)
	if err != nil {
		return s, err
	}
	for _, st := range sts {
		s += fmt.Sprintf("%v %v %v\n", indent, st.Name, st.Tqid())
		if st.Tmode().IsDir() {
			s1, err := fsl.sprintfDirIndent(d+"/"+st.Name, indent+" ")
			if err != nil {
				return s, err
			}
			s += s1
		}
	}
	return s, nil
}
