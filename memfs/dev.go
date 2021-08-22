package memfs

import (
	"fmt"
	"time"

	"ulambda/fs"
	np "ulambda/ninep"
)

type Dev interface {
	Write(np.Toffset, []byte) (np.Tsize, error)
	Read(np.Toffset, np.Tsize) ([]byte, error)
	Len() np.Tlength
}

type Device struct {
	*Inode
	d Dev
}

func MakeDev(i *Inode) *Device {
	d := Device{}
	d.Inode = i
	return &d
}

func (d *Device) Size() np.Tlength {
	return d.d.Len()
}

func (d *Device) SetParent(p *Dir) {
	d.parent = p
}

func (d *Device) Stat(ctx fs.CtxI) (*np.Stat, error) {
	d.Lock()
	defer d.Unlock()
	st := d.Inode.stat()
	st.Length = d.d.Len()
	return st, nil
}

func (d *Device) Write(ctx fs.CtxI, offset np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	d.Lock()
	defer d.Unlock()
	if v != np.NoV && d.version != v {
		return 0, fmt.Errorf("Version mismatch")
	}
	d.version += 1
	d.Mtime = time.Now().Unix()
	return d.d.Write(offset, data)
}

func (d *Device) Read(ctx fs.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]byte, error) {
	d.Lock()
	defer d.Unlock()

	if v != np.NoV && d.version != v {
		return nil, fmt.Errorf("Version mismatch")
	}
	if offset >= np.Toffset(d.Size()) {
		return nil, nil
	}
	return d.d.Read(offset, n)
}
