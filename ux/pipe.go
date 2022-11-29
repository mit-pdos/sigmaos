package fsux

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/sigmap"
	"sigmaos/pipe"
)

type Pipe struct {
	*pipe.Pipe
	*Obj
}

func makePipe(ctx fs.CtxI, pathName np.Path) (*Pipe, *np.Err) {
	p := &Pipe{}
	o, err := makeObj(pathName)
	if err != nil {
		return nil, err
	}
	p.Pipe = pipe.MakePipe(ctx)
	p.Obj = o
	return p, nil
}

func (p *Pipe) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	db.DPrintf("UXD", "%v: PipeOpen %v %v path %v flags %v\n", ctx, p, m, p.Path(), uxFlags(m))
	pr := fsux.ot.AllocRef(p.Obj.path, p).(*Pipe)
	if _, err := pr.Pipe.Open(ctx, m); err != nil {
		return nil, err
	}
	return pr, nil
}

func (p *Pipe) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	db.DPrintf("UXD", "%v: PipeClose path %v\n", ctx, p.Path())
	pr := fsux.ot.AllocRef(p.Obj.path, p).(*Pipe)
	fsux.ot.Clunk(p.Obj.path)
	return pr.Pipe.Close(ctx, mode)
}

func (p *Pipe) Unlink() {
	p.Pipe.Unlink()
}
