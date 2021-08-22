package npux

import (
	"io"
	"os"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

type File struct {
	*Obj
	file *os.File
}

func (npux *NpUx) makeFile(path []string, t np.Tperm, p *Dir) *File {
	f := &File{}
	npux.mu.Lock()
	defer npux.mu.Unlock()
	f.Obj = npux.makeObjL(path, t, p)
	return f
}

func (f *File) Open(ctx fs.CtxI, m np.Tmode) error {
	db.DLPrintf("UXD", "%v: Open %v %v path %v\n", ctx, f, m, f.Path())
	file, err := os.OpenFile(f.Path(), uxFlags(m), 0)
	if err != nil {
		return err
	}
	f.file = file
	return nil
}

func (f *File) uxWrite(off int64, b []byte) (np.Tsize, error) {
	db.DLPrintf("UXD", "%v: WriteFile: off %v cnt %v %v\n", f, off, len(b), f.file)
	_, err := f.file.Seek(off, 0)
	if err != nil {
		return 0, err
	}
	n, err := f.file.Write(b)
	return np.Tsize(n), err
}

func (f *File) uxRead(off int64, cnt int) ([]byte, error) {
	b := make([]byte, cnt)
	_, err := f.file.Seek(off, 0)
	if err != nil {
		return nil, err
	}
	n, err := f.file.Read(b)
	if err == io.EOF {
		return b[:n], nil
	}
	if err != nil {
		return nil, err
	}
	return b[:n], err
}

func (f *File) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	db.DLPrintf("UXD", "%v: Read: %v off %v cnt %v\n", ctx, f, off, cnt)
	b, err := f.uxRead(int64(off), int(cnt))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (f *File) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	db.DLPrintf("UXD", "%v: Write %v off %v sz %v\n", ctx, f, off, len(b))
	return f.uxWrite(int64(off), b)
}
