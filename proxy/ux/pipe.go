package fsux

import (
	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/path"
	"sigmaos/sigmasrv/pipe"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Pipe struct {
	*pipe.Pipe
	*Obj
}

func newPipe(ctx fs.CtxI, pathName path.Tpathname) (*Pipe, *serr.Err) {
	p := &Pipe{}
	o, err := newObj(pathName)
	if err != nil {
		return nil, err
	}
	p.Pipe = pipe.NewPipe(ctx)
	p.Obj = o
	return p, nil
}

func (p *Pipe) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.UX, "%v: PipeOpen %v %v path %v flags %v\n", ctx, p, m, p.Path(), uxFlags(m))
	pr := fsux.ot.AllocRef(p.Obj.path, p).(*Pipe)
	if _, err := pr.Pipe.Open(ctx, m); err != nil {
		return nil, err
	}
	return pr, nil
}

func (p *Pipe) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	db.DPrintf(db.UX, "%v: PipeClose path %v\n", ctx, p.Path())
	pr := fsux.ot.AllocRef(p.Obj.path, p).(*Pipe)
	fsux.ot.Clunk(p.Obj.path)
	return pr.Pipe.Close(ctx, mode)
}

func (p *Pipe) Unlink() {
	p.Pipe.Unlink()
}
