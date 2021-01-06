package fs

import (
	np "ulambda/ninep"
)

type File struct {
	data []byte
}

func MakeFile() *File {
	f := &File{make([]byte, 0)}
	return f
}

func (f *File) Len() np.Tlength {
	return np.Tlength(len(f.data))
}

func (f *File) Write(offset np.Toffset, data []byte) (np.Tsize, error) {
	f.data = data
	return np.Tsize(len(data)), nil
}

func (f *File) Read(np.Toffset, np.Tsize) ([]byte, error) {
	return f.data, nil
}
