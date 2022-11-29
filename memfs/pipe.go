package memfs

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/sigmap"
	"sigmaos/pipe"
)

type Pipe struct {
	fs.Inode
	*pipe.Pipe
}

func MakePipe(ctx fs.CtxI, i fs.Inode) *Pipe {
	p := Pipe{}
	p.Pipe = pipe.MakePipe(ctx)
	p.Inode = i
	return &p
}

func (p *Pipe) Size() (np.Tlength, *np.Err) {
	return p.Pipe.Size(), nil
}

func (p *Pipe) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	return p.Pipe.Close(ctx, m)
}

func (p *Pipe) Open(ctx fs.CtxI, mode np.Tmode) (fs.FsObj, *np.Err) {
	return p.Pipe.Open(ctx, mode)
}

func (p *Pipe) Unlink() {
	p.Pipe.Unlink()
}

func (p *Pipe) Snapshot(fn fs.SnapshotF) []byte {
	db.DFatalf("tried to snapshot pipe")
	return nil
}

func (p *Pipe) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	st, err := p.Inode.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = p.Pipe.Size()
	return nil, nil
}
