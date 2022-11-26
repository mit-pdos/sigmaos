package dbd

import (
	"database/sql"

	"sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	np "sigmaos/ninep"
)

const (
	CLONEFDEV = "clonefdev"
	QUERY     = "query"
)

type fileDev struct {
	dbaddr string
	mfs    *memfssrv.MemFs
}

func mkFileDev(dbaddr string, mfs *memfssrv.MemFs) *fileDev {
	return &fileDev{dbaddr, mfs}
}

type fileSession struct {
	*inode.Inode
	id     np.Tsession
	res    []byte
	dbaddr string
}

// XXX wait on close before processing data?
func (fs *fileSession) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	debug.DPrintf("DBSRV", "doQuery: %v\n", string(b))
	db, error := sql.Open("mysql", "sigma:sigmaos@tcp("+fs.dbaddr+")/books")
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
func (fd *fileDev) mkSession(mfs *memfssrv.MemFs, sid np.Tsession) *np.Err {
	fs := &fileSession{}
	fs.id = sid
	fs.dbaddr = fd.dbaddr
	i, err := mfs.MkDev(sid.String()+"/"+QUERY, fs)
	if err != nil {
		return err
	}
	fs.Inode = i
	return nil
}

func (fd *fileDev) detachSession(sid np.Tsession) {
	if err := fd.mfs.Remove(sid.String() + "/" + QUERY); err != nil {
		debug.DPrintf("DBSRV", "detachSessoin err %v\n", err)
	}
}
