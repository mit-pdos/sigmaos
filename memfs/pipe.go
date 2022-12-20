package memfs

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/pipe"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
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

func (p *Pipe) Size() (sp.Tlength, *serr.Err) {
	return p.Pipe.Size(), nil
}

func (p *Pipe) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	return p.Pipe.Close(ctx, m)
}

func (p *Pipe) Open(ctx fs.CtxI, mode sp.Tmode) (fs.FsObj, *serr.Err) {
	return p.Pipe.Open(ctx, mode)
}

func (p *Pipe) Unlink() {
	p.Pipe.Unlink()
}

func (p *Pipe) Snapshot(fn fs.SnapshotF) []byte {
	db.DFatalf("tried to snapshot pipe")
	return nil
}

func (p *Pipe) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := p.Inode.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = uint64(p.Pipe.Size())
	return nil, nil
}
