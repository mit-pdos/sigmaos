package protdevsrv

import (
	"log"
	"reflect"

	db "sigmaos/debug"
	// "sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	np "sigmaos/ninep"
	"sigmaos/proc"
)

const (
	STATS = "stats"
	CLONE = "clone"
	RPC   = "rpc"
	CTL   = "ctl"
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

type rpcSession struct {
	*inode.Inode
	id np.Tsession
}

func mkRpcSession(psd *ProtDevSrv, sid np.Tsession) (*rpcSession, *np.Err) {
	s := &rpcSession{}
	s.id = sid
	i, err := psd.MemFs.MkDev(sid.String()+"/"+CTL, s)
	if err != nil {
		return nil, err
	}
	s.Inode = i
	if err := mkRPCDev(sid.String(), psd); err != nil {
		return nil, err
	}
	return s, nil
}

func (sc *rpcSession) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}
	return []byte(sc.id.String()), nil
}

func (sc *rpcSession) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, nil)
}

func (sc *rpcSession) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("PROTDEVSRV", "Close ctl %v\n", sc.id)
	return nil
}

type Clone struct {
	*inode.Inode
	psd *ProtDevSrv
}

func makeClone(mfs *memfssrv.MemFs, psd *ProtDevSrv) *np.Err {
	cl := &Clone{}
	cl.psd = psd
	i, err := psd.MemFs.MkDev(CLONE, cl) // put clone file into root dir
	if err != nil {
		return err
	}
	cl.Inode = i
	return nil
}

func (c *Clone) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, *np.Err) {
	sid := ctx.SessionId()
	db.DPrintf("PROTDEVSRV", "%v: Open clone dir %v\n", proc.GetProgram(), sid)
	if _, err := c.psd.MemFs.Create(sid.String(), np.DMDIR, np.ORDWR); err != nil {
		return nil, err
	}
	return mkRpcSession(c.psd, sid)
}

func (c *Clone) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("PROTDEVSRV", "%v: Close clone\n", proc.GetProgram())
	return nil
}

type ProtDevSrv struct {
	*memfssrv.MemFs
	sti *StatInfo
	svc *service
}

func MakeProtDevSrv(fn string, svci any) (*ProtDevSrv, error) {
	psd := &ProtDevSrv{}
	mfs, _, _, error := memfssrv.MakeMemFsDetach(fn, "protdevsrv", psd.Detach)
	if error != nil {
		db.DFatalf("protdevsrv.Run: %v\n", error)
	}
	psd.MemFs = mfs
	psd.mkService(svci)
	if err := makeClone(mfs, psd); err != nil {
		return nil, err
	}
	if si, err := makeStatsDev(mfs); err != nil {
		return nil, err
	} else {
		psd.sti = si
	}
	return psd, nil
}

func (psd *ProtDevSrv) QueueLen() int {
	return psd.MemFs.QueueLen()
}

func (psd *ProtDevSrv) Detach(ctx fs.CtxI, session np.Tsession) {
	db.DPrintf("PROTDEVSRV", "Detach %v %p %v\n", session, psd.MemFs, psd.MemFs.Root())
	dir := session.String() + "/"
	if err := psd.MemFs.RemoveXXX(dir + CTL); err != nil {
		db.DPrintf("PROTDEVSRV", "Remove ctl err %v\n", err)
	}
	if err := psd.MemFs.RemoveXXX(dir + RPC); err != nil {
		db.DPrintf("PROTDEVSRV", "Remove rpc err %v\n", err)
	}
	if err := psd.MemFs.RemoveXXX(dir); err != nil {
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
	db.DPrintf("PROTDEVSRV", "Run %v\n", proc.GetProgram())
	psd.MemFs.Serve()
	psd.MemFs.Done()
	return nil
}
