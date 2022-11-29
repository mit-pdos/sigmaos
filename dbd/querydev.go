package dbd

import (
	"database/sql"

	"sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	np "sigmaos/sigmap"
)

type queryDev struct {
	dbaddr string
}

type fileSession struct {
	*inode.Inode
	id     np.Tsession
	dbaddr string
	res    []byte
}

// XXX wait on close before processing data?
func (fs *fileSession) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	debug.DPrintf("DBSRV", "doQuery: %v\n", string(b))
	db, error := sql.Open("mysql", "sigma:sigmaos@tcp("+fs.dbaddr+")/sigmaos")
	if error != nil {
		return 0, np.MkErrError(error)
	}
	res, err := doQuery(db, string(b))
	if err != nil {
		return 0, np.MkErrError(err)
	}
	fs.res = res

	if err := db.Close(); err != nil {
		return 0, np.MkErrError(err)
	}

	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (fs *fileSession) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}
	return fs.res, nil
}

// XXX clean up in case of error
func (qd *queryDev) mkSession(mfs *memfssrv.MemFs, sid np.Tsession) (fs.Inode, *np.Err) {
	fs := &fileSession{}
	fs.Inode = mfs.MakeDevInode()
	fs.id = sid
	fs.dbaddr = qd.dbaddr
	return fs, nil
}
