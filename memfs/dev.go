package memfs

import (
	"fmt"

	"ulambda/fs"
	_ "ulambda/inode"
	np "ulambda/ninep"
)

type Dev interface {
	Write(np.Toffset, []byte) (np.Tsize, error)
	Read(np.Toffset, np.Tsize) ([]byte, error)
	Len() np.Tlength
}

type Device struct {
	fs.FsObj
	d Dev
}

func MakeDev(i fs.FsObj) *Device {
	dev := Device{}
	dev.FsObj = i
	return &dev
}

func (d *Device) Size() np.Tlength {
	return d.d.Len()
}

func (d *Device) Stat(ctx fs.CtxI) (*np.Stat, error) {
	d.Lock()
	defer d.Unlock()
	st, err := d.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = d.d.Len()
	return st, nil
}

func (d *Device) Write(ctx fs.CtxI, offset np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	d.Lock()
	defer d.Unlock()
	if v != np.NoV && d.Version() != v {
		return 0, fmt.Errorf("Version mismatch")
	}
	d.VersionInc()
	d.SetMtime()
	return d.d.Write(offset, data)
}

func (d *Device) Read(ctx fs.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]byte, error) {
	d.Lock()
	defer d.Unlock()

	if v != np.NoV && d.Version() != v {
		return nil, fmt.Errorf("Version mismatch")
	}
	if offset >= np.Toffset(d.Size()) {
		return nil, nil
	}
	return d.d.Read(offset, n)
}
