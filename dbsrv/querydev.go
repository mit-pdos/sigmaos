package dbsrv

import (
	"database/sql"

	"sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type queryDev struct {
	dbaddr string
}

type fileSession struct {
	*inode.Inode
	id     sessp.Tsession
	dbaddr string
	res    []byte
}

func (fs *fileSession) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := fs.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	st.SetLengthInt(len(fs.res))
	return st, nil
}

// XXX wait on close before processing data?
func (fs *fileSession) Write(ctx fs.CtxI, off sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	debug.DPrintf(debug.DB, "doQuery: %v\n", string(b))
	db, error := sql.Open("mysql", "sigma:sigmaos@tcp("+fs.dbaddr+")/sigmaos")
	if error != nil {
		return 0, serr.NewErrError(error)
	}
	res, err := doQuery(db, string(b))
	if err != nil {
		return 0, serr.NewErrError(err)
	}
	fs.res = res

	if err := db.Close(); err != nil {
		return 0, serr.NewErrError(err)
	}

	return sp.Tsize(len(b)), nil
}

// XXX incremental read
func (fs *fileSession) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	return fs.res, nil
}

// XXX clean up in case of error
func (qd *queryDev) newSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.FsObj, *serr.Err) {
	fs := &fileSession{}
	fs.Inode = mfs.NewDevInode()
	fs.id = sid
	fs.dbaddr = qd.dbaddr
	return fs, nil
}
