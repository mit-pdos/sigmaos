package pobj

import (
	"fmt"
	"sigmaos/fs"
	"sigmaos/path"
)

type Pobj struct {
	pathname path.Tpathname
	obj      fs.FsObj
	ctx      fs.CtxI
}

func NewPobj(pn path.Tpathname, o fs.FsObj, ctx fs.CtxI) *Pobj {
	return &Pobj{pn, o, ctx}
}

func (po *Pobj) String() string {
	return fmt.Sprintf("{%v %v %v}", po.pathname, po.obj, po.ctx)
}

func (po *Pobj) Pathname() path.Tpathname {
	return po.pathname
}

func (po *Pobj) Ctx() fs.CtxI {
	return po.ctx
}

func (po *Pobj) SetPath(path path.Tpathname) {
	po.pathname = path
}

func (po *Pobj) Obj() fs.FsObj {
	return po.obj
}

func (po *Pobj) SetObj(o fs.FsObj) {
	po.obj = o
}