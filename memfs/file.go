package memfs

import (
	"sync"

	np "ulambda/ninep"
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

func (f *File) Len() np.Tlength {
	f.mu.Lock()
	defer f.mu.Unlock()
	return np.Tlength(len(f.data))
}

// Caller must hold lock
func (f *File) LenOff() np.Toffset {
	return np.Toffset(len(f.data))
}

func (f *File) write(offset np.Toffset, data []byte) (np.Tsize, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

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

func (f *File) read(offset np.Toffset, n np.Tsize) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if offset >= f.LenOff() {
		return nil, nil
	} else {
		end := offset + np.Toffset(n)
		if end >= f.LenOff() {
			end = f.LenOff()
		}
		return f.data[offset:end], nil
	}

}
