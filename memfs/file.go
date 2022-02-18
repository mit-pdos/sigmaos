package memfs

import (
	"sync"
	"time"

	"ulambda/fs"
	np "ulambda/ninep"
)

type File struct {
	fs.FsObj
	mu   sync.Mutex
	data []byte
}

func MakeFile(i fs.FsObj) *File {
	f := &File{}
	f.FsObj = i
	f.data = make([]byte, 0)
	return f
}

func (f *File) Size() np.Tlength {
	f.mu.Lock()
	defer f.mu.Unlock()
	return np.Tlength(len(f.data))
}

func (f *File) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, err := f.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = np.Tlength(len(f.data))
	return st, nil
}

func (f *File) LenOff() np.Toffset {
	return np.Toffset(len(f.data))
}

func (f *File) Write(ctx fs.CtxI, offset np.Toffset, data []byte, v np.TQversion) (np.Tsize, *np.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !np.VEq(v, f.Version()) {
		return 0, np.MkErr(np.TErrVersion, f.Version())
	}

	f.VersionInc()
	f.SetMtime(time.Now().Unix())

	cnt := np.Tsize(len(data))
	sz := np.Toffset(len(data))
	if offset >= f.LenOff() { // passed end of file?
		n := f.LenOff() - offset
		f.data = append(f.data, make([]byte, n)...)
		f.data = append(f.data, data...)
		return cnt, nil
	}

	var d []byte
	if offset+sz < f.LenOff() { // in the middle of the file?
		d = f.data[offset+sz:]
	}
	f.data = f.data[0:offset]
	f.data = append(f.data, data...)
	f.data = append(f.data, d...)
	return cnt, nil
}

func (f *File) Read(ctx fs.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !np.VEq(v, f.Version()) {
		return nil, np.MkErr(np.TErrVersion, f.Version())
	}
	if offset >= f.LenOff() {
		return nil, nil
	} else {
		// XXX overflow?
		end := offset + np.Toffset(n)
		if end >= f.LenOff() {
			end = f.LenOff()
		}
		b := f.data[offset:end]
		return b, nil
	}
}
