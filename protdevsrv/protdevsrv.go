package protdevsrv

import (
	"log"
	"reflect"

	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/inode"
	np "sigmaos/ninep"
)

const (
	STATS = "stats"
	CLONE = "clone"
)

//
// RPC server, which borrows from go's RPC dispatch
//

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

type Request struct {
	Method string
	Args   []byte
}

type Reply struct {
	Res   []byte
	Error string
}

type method struct {
	method    reflect.Method
	argType   reflect.Type
	replyType reflect.Type
}

type service struct {
	svc     reflect.Value
	typ     reflect.Type
	methods map[string]*method
}

type stream struct {
	*inode.Inode
	fs.RPC
}

type streamCtl struct {
	*inode.Inode
	id np.Tsession
}

func mkStreamCtl(mfs *fslibsrv.MemFs, sid np.Tsession) *streamCtl {
	s := &streamCtl{}
	s.id = sid
	s.Inode = mfs.MakeInode(np.DMTMP) // XXX root
	return s
}

func (sc *streamCtl) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}
	return []byte(sc.id.String()), nil
}

func (sc *streamCtl) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, nil)
}

func (sc *streamCtl) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("PROTDEVSRV", "Close ctl %v\n", sc.id)
	return nil
}

type Clone struct {
	fs.Inode
	psd *ProtDevSrv
}

func makeClone(mfs *fslibsrv.MemFs, psd *ProtDevSrv) fs.Inode {
	i := mfs.MakeInode(np.DMDEVICE)
	return &Clone{i, psd}
}

func (c *Clone) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {

	// XXX should pass in directory

	db.DPrintf("PROTDEVSRV", "Open clone: create dir %v\n", ctx.SessionId())

	i, err := c.psd.MemFs.Create(ctx.SessionId().String(), np.DMDIR|np.DMTMP, np.ORDWR)
	d := i.(fs.Dir)

	sc := mkStreamCtl(c.psd.MemFs, ctx.SessionId())
	err = dir.MkNod(ctx, d, "ctl", sc) // put ctl file into stream dir
	if err != nil {
		db.DFatalf("MkNod err %v\n", err)
	}

	// make data/stream file
	st := &stream{}
	st.Inode = inode.MakeInode(nil, np.DMTMP, d)
	st.RPC, err = mkStream(c.psd)
	if err != nil {
		db.DFatalf("mkStream err %v\n", err)
	}
	dir.MkNod(ctx, d, "data", st)

	return sc, nil
}

func (c *Clone) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("PROTDEVSRV", "Close clone\n")
	return nil
}

type ProtDevSrv struct {
	*fslibsrv.MemFs
	sts *StatInfo
	svc *service
}

func MakeProtDevSrv(fn string, svci any) *ProtDevSrv {
	psd := &ProtDevSrv{}
	mfs, _, _, error := fslibsrv.MakeMemFsDetach(fn, "protdevsrv", psd.Detach)
	if error != nil {
		db.DFatalf("protdevsrv.Run: %v\n", error)
	}
	psd.MemFs = mfs
	psd.mkService(svci)
	err := mfs.MkNod(CLONE, makeClone(mfs, psd))
	if err != nil {
		db.DFatalf("MakeNod clone failed %v\n", err)
	}
	psd.sts = MkStats()
	err = mfs.MkNod(STATS, makeStatsDev(nil, mfs.Root(), psd.sts))
	if err != nil {
		db.DFatalf("MakeNod stats failed %v\n", err)
	}
	return psd
}

func (psd *ProtDevSrv) QueueLen() int {
	return psd.MemFs.QueueLen()
}

func (psd *ProtDevSrv) Detach(ctx fs.CtxI, session np.Tsession) {
	db.DPrintf("PROTDEVSRV", "Detach %v %p %v\n", session, psd.MemFs, psd.MemFs.Root())
	root := psd.MemFs.Root()
	_, o, _, err := root.LookupPath(nil, np.Path{session.String()})
	if err != nil {
		db.DPrintf("PROTDEVSRV", "LookupPath err %v\n", err)
	}
	d := o.(fs.Dir)
	err = d.Remove(nil, "ctl")
	if err != nil {
		db.DPrintf("PROTDEVSRV", "Remove ctl err %v\n", err)
	}
	err = d.Remove(nil, "data")
	if err != nil {
		db.DPrintf("PROTDEVSRV", "Remove data err %v\n", err)
	}
	err = root.Remove(nil, session.String())
	if err != nil {
		db.DPrintf("PROTDEVSRV", "Detach err %v\n", err)
	}
}

func (psd *ProtDevSrv) mkService(svci any) {
	svc := &service{}
	svc.typ = reflect.TypeOf(svci)
	svc.svc = reflect.ValueOf(svci)
	svc.methods = map[string]*method{}

	for m := 0; m < svc.typ.NumMethod(); m++ {
		methodt := svc.typ.Method(m)
		mtype := methodt.Type
		mname := methodt.Name

		// log.Printf("%v pp %v ni %v no %v\n", mname, methodt.PkgPath, mtype.NumIn(), mtype.NumOut())
		if methodt.PkgPath != "" || // capitalized?
			mtype.NumIn() != 3 ||
			//mtype.In(1).Kind() != reflect.Ptr ||
			mtype.In(2).Kind() != reflect.Ptr ||
			mtype.NumOut() != 1 ||
			mtype.Out(0) != typeOfError {
			// the method is not suitable for a handler
			log.Printf("bad method: %v\n", mname)
		} else {
			// the method looks like a handler
			svc.methods[mname] = &method{methodt, mtype.In(1), mtype.In(2)}
		}
	}
	psd.svc = svc
}

func (psd *ProtDevSrv) RunServer() error {
	psd.MemFs.Serve()
	psd.MemFs.Done()
	return nil
}
