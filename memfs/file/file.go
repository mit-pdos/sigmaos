package file

import (
	"sync"
	//"time"

	db "sigmaos/debug"
	"sigmaos/fencefs"
	"sigmaos/fs"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type File struct {
	mu   sync.Mutex
	data []byte
}

func NewFile() *File {
	f := &File{}
	f.data = make([]byte, 0)
	return f
}

func (f *File) Size() (sp.Tlength, *serr.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sp.Tlength(len(f.data)), nil
}

func (f *File) LenOff() sp.Toffset {
	return sp.Toffset(len(f.data))
}

func (f *File) write(ctx fs.CtxI, offset sp.Toffset, data []byte) (sp.Tsize, *serr.Err) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// f.SetMtime(time.Now().Unix())

	cnt := sp.Tsize(len(data))
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

func (f *File) Write(ctx fs.CtxI, offset sp.Toffset, data []byte, fence sp.Tfence) (sp.Tsize, *serr.Err) {
	db.DPrintf(db.FENCEFS, "File.Write %v %v\n", fence, ctx.FenceFs())
	if fi, err := fencefs.CheckFence(ctx.FenceFs(), fence); err != nil {
		return 0, err
	} else {
		if fi == nil {
			return f.write(ctx, offset, data)
		} else {
			defer fi.RUnlock()
			return f.write(ctx, offset, data)
		}
	}
}

func (f *File) Read(ctx fs.CtxI, offset sp.Toffset, n sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
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
