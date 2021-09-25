package memfs

import (
	"fmt"

	"ulambda/fs"
	np "ulambda/ninep"
)

type File struct {
	fs.FsObj
	data []byte
}

func MakeFile(i fs.FsObj) *File {
	return &File{i, make([]byte, 0)}
}

func (f *File) Size() np.Tlength {
	f.Lock()
	defer f.Unlock()
	return np.Tlength(len(f.data))
}

func (f *File) Stat(ctx fs.CtxI) (*np.Stat, error) {
	f.Lock()
	defer f.Unlock()
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

func (f *File) Write(ctx fs.CtxI, offset np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	f.Lock()
	defer f.Unlock()

	if v != np.NoV && f.Version() != v {
		return 0, fmt.Errorf("Version mismatch")
	}

	f.VersionInc()
	f.SetMtime()

	cnt := np.Tsize(len(data))
	sz := np.Toffset(len(data))
	if offset > f.LenOff() { // passed end of file?
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

func (f *File) Read(ctx fs.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]byte, error) {
	f.Lock()
	defer f.Unlock()

	if v != np.NoV && f.Version() != v {
		return nil, fmt.Errorf("Version mismatch")
	}

	if offset >= f.LenOff() {
		return nil, nil
	} else {
		end := offset + np.Toffset(n)
		if end >= f.LenOff() {
			end = f.LenOff()
		}
		b := f.data[offset:end]
		return b, nil
	}
}
