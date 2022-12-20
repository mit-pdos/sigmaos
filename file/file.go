package file

import (
	"sync"
	//"time"

	"sigmaos/fs"
	sp "sigmaos/sigmap"
    "sigmaos/fcall"
)

type File struct {
	mu   sync.Mutex
	data []byte
}

func MakeFile() *File {
	f := &File{}
	f.data = make([]byte, 0)
	return f
}

func (f *File) Size() (sp.Tlength, *fcall.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sp.Tlength(len(f.data)), nil
}

func (f *File) LenOff() sp.Toffset {
	return sp.Toffset(len(f.data))
}

func (f *File) Write(ctx fs.CtxI, offset sp.Toffset, data []byte, v sp.TQversion) (fcall.Tsize, *fcall.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// f.SetMtime(time.Now().Unix())

	cnt := fcall.Tsize(len(data))
	sz := sp.Toffset(len(data))
	if offset == sp.NoOffset { // OAPPEND
		offset = f.LenOff()
	}

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

func (f *File) Read(ctx fs.CtxI, offset sp.Toffset, n fcall.Tsize, v sp.TQversion) ([]byte, *fcall.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if offset >= f.LenOff() {
		return nil, nil
	} else {
		// XXX overflow?
		end := offset + sp.Toffset(n)
		if end >= f.LenOff() {
			end = f.LenOff()
		}
		b := f.data[offset:end]
		return b, nil
	}
}

func (f *File) Snapshot() []byte {
	return f.data
}

func RestoreFile(d []byte) *File {
	f := MakeFile()
	f.data = d
	return f
}
