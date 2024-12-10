package memfs

import (
	"sigmaos/api/fs"
	"sigmaos/sigmasrv/pipe"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Pipe struct {
	fs.Inode
	*pipe.Pipe
}

func NewPipe(ctx fs.CtxI, i fs.Inode) *Pipe {
	p := Pipe{}
	p.Pipe = pipe.NewPipe(ctx)
	p.Inode = i
	return &p
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

func (p *Pipe) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
	st, err := p.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	st.SetLength(p.Pipe.Size())
	return nil, nil
}
