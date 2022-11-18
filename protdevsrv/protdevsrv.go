package protdevsrv

import (
	"log"
	"reflect"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/inode"
	"sigmaos/memfs"
	np "sigmaos/ninep"
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

func mkService(svci any) *service {
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
	return svc
}

type stream struct {
	*inode.Inode
	fs.File
}

type streamCtl struct {
	*inode.Inode
	id np.Tsession
}

func mkStreamCtl(sid np.Tsession) *streamCtl {
	s := &streamCtl{}
	s.id = sid
	s.Inode = inode.MakeInode(nil, np.DMTMP, nil)
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
	psd *ProtSrvDev
}

func makeClone(ctx fs.CtxI, root fs.Dir, psd *ProtSrvDev) fs.Inode {
	i := inode.MakeInode(ctx, np.DMDEVICE, root)
	return &Clone{i, psd}
}

func (c *Clone) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	sc := mkStreamCtl(ctx.SessionId())

	db.DPrintf("PROTDEVSRV", "Open clone: create dir %v\n", sc.id)

	// create directory for stream
	di := inode.MakeInode(nil, np.DMDIR|np.DMTMP, c.Parent())
	d := dir.MakeDir(di, memfs.MakeInode)
	err := dir.MkNod(ctx, c.Parent(), sc.id.String(), d)
	if err != nil {
		db.DFatalf("MkNod d %v err %v\n", d, err)
	}
	err = dir.MkNod(ctx, d, "ctl", sc) // put ctl file into stream dir
	if err != nil {
		db.DFatalf("MkNod err %v\n", err)
	}

	// make data/stream file
	st := &stream{}
	st.Inode = inode.MakeInode(nil, np.DMTMP, d)
	st.File, err = mkStream(c.psd.sts, c.psd.svc)
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

type ProtSrvDev struct {
	*fslibsrv.MemFs
	sts *Stats
	svc *service
}

func (psd *ProtSrvDev) Detach(ctx fs.CtxI, session np.Tsession) {
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

func MakeProtDevSrv(fn string, svci any) *ProtSrvDev {
	psd := &ProtSrvDev{}
	mfs, _, _, error := fslibsrv.MakeMemFsDetach(fn, "protdevsrv", psd.Detach)
	if error != nil {
		db.DFatalf("protdevsrv.Run: %v\n", error)
	}
	psd.MemFs = mfs
	psd.svc = mkService(svci)
	err := dir.MkNod(ctx.MkCtx("", 0, nil), mfs.Root(), "clone", makeClone(nil, mfs.Root(), psd))
	if err != nil {
		db.DFatalf("MakeNod clone failed %v\n", err)
	}
	psd.sts = MkStats()
	err = dir.MkNod(ctx.MkCtx("", 0, nil), mfs.Root(), "stats", makeStatsDev(nil, mfs.Root(), psd.sts))
	if err != nil {
		db.DFatalf("MakeNod clone failed %v\n", err)
	}
	return psd
}

func (psd *ProtSrvDev) RunServer() error {
	psd.MemFs.Serve()
	psd.MemFs.Done()
	return nil
}
