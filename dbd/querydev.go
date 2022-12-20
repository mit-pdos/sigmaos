package dbd

import (
	"database/sql"

	"sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
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

// XXX wait on close before processing data?
func (fs *fileSession) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	debug.DPrintf(debug.DB, "doQuery: %v\n", string(b))
	db, error := sql.Open("mysql", "sigma:sigmaos@tcp("+fs.dbaddr+")/sigmaos")
	if error != nil {
		return 0, serr.MkErrError(error)
	}
	res, err := doQuery(db, string(b))
	if err != nil {
		return 0, serr.MkErrError(err)
	}
	fs.res = res

	if err := db.Close(); err != nil {
		return 0, serr.MkErrError(err)
	}

	return sessp.Tsize(len(b)), nil
}

// XXX incremental read
func (fs *fileSession) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	return fs.res, nil
}

// XXX clean up in case of error
func (qd *queryDev) mkSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	fs := &fileSession{}
	fs.Inode = mfs.MakeDevInode()
	fs.id = sid
	fs.dbaddr = qd.dbaddr
	return fs, nil
}
