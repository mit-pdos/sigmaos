package fences1

import (
	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

// XXX Take any hash table and export it?

func (ft *FenceTable) Lookup(ctx fs.CtxI, path np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DLPrintf("FENCEDIR", "Lookup %v\n", path)
	if len(path) == 0 {
		return nil, nil, nil, nil
	}
	p, err := np.String2Path(path[0])
	if err != nil {
		return nil, nil, path, np.MkErr(np.TErrError, err)
	}
	ft.Lock()
	defer ft.Unlock()
	if e, ok := ft.fences[p]; ok {
		qids := []np.Tqid{e.Qid()}
		if len(path) > 1 {
			return qids, e, path[1:], np.MkErr(np.TErrNotfound, path[1])
		}
		return qids, e, nil, nil
	}
	return nil, nil, path, np.MkErr(np.TErrNotfound, path[0])
}

func (ft *FenceTable) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, name)
}

// XXX account for extra entries in cursor, and sort
// XXX ignores size
func (ft *FenceTable) ReadDir(ctx fs.CtxI, cursor int, n np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	db.DLPrintf("FENCEDIR", "ReadDir FT %v\n", ft.fences)
	ft.Lock()
	defer ft.Unlock()

	if cursor > 0 {
		return nil, nil
	}
	sts := make([]*np.Stat, 0, len(ft.fences))
	for p, _ := range ft.fences {
		st := &np.Stat{}
		st.Name = p.String()
		sts = append(sts, st)
	}
	return sts, nil
}

func (ft *FenceTable) WriteDir(ctx fs.CtxI, offset np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, "writedir")
}

func (ft *FenceTable) Rename(ctx fs.CtxI, from, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "rename")
}

func (ft *FenceTable) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "renameat")
}

func (ft *FenceTable) Remove(ctx fs.CtxI, n string) *np.Err {
	db.DLPrintf("FENCEDIR", "Remove n %v\n", n)

	p, err := np.String2Path(n)
	if err != nil {
		return np.MkErr(np.TErrError, err)
	}
	ft.Lock()
	defer ft.Unlock()
	delete(ft.fences, p)

	return nil
}
