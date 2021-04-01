package memfs

import (
	"fmt"
	"time"

	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
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

func (d *Device) Stat(ctx npo.CtxI) (*np.Stat, error) {
	d.Lock()
	defer d.Unlock()
	st := d.Inode.stat()
	st.Length = d.d.Len()
	return st, nil
}

func (d *Device) Open(ctx npo.CtxI, mode np.Tmode) error {
	return nil
}

func (d *Device) Close(ctx npo.CtxI, mode np.Tmode) error {
	return nil
}

func (d *Device) Write(ctx npo.CtxI, offset np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	if v != np.NoV && d.version != v {
		return 0, fmt.Errorf("Version mismatch")
	}
	d.version += 1
	d.Mtime = time.Now().Unix()
	return d.d.Write(offset, data)
}

func (d *Device) Read(ctx npo.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]byte, error) {
	if v != np.NoV && d.version != v {
		return nil, fmt.Errorf("Version mismatch")
	}
	if offset >= np.Toffset(d.Size()) {
		return nil, nil
	}
	return d.d.Read(offset, n)
}
