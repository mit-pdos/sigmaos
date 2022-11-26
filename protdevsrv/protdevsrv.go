package protdevsrv

import (
	"log"
	"reflect"

	"sigmaos/clonedev"
	db "sigmaos/debug"
	"sigmaos/fs"
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
	rd := mkRpcDev(psd)
	if err := clonedev.MkCloneDev(psd.MemFs, CLONE, rd.mkRpcSession); err != nil {
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
	if err := psd.MemFs.Remove(dir + CTL); err != nil {
		db.DPrintf("PROTDEVSRV", "Remove ctl err %v\n", err)
	}
	if err := psd.MemFs.Remove(dir + RPC); err != nil {
		db.DPrintf("PROTDEVSRV", "Remove rpc err %v\n", err)
	}
	if err := psd.MemFs.Remove(dir); err != nil {
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
