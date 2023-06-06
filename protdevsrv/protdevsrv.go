package protdevsrv

import (
	"log"
	"reflect"

	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/protdev"
	"sigmaos/sessdevsrv"
	"sigmaos/sigmaclnt"
)

//
// RPC server, which borrows from go's RPC dispatch
//

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

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
	sti *protdev.StatInfo
	svc *service
}

func MakeProtDevSrv(fn string, svci any) (*ProtDevSrv, error) {
	mfs, error := memfssrv.MakeMemFs(fn, "protdevsrv")
	if error != nil {
		db.DFatalf("protdevsrv.Run [%v]: %v\n", fn, error)
	}
	return MakeProtDevSrvMemFs(mfs, svci)
}

func MakeProtDevSrvPublic(fn string, svci any, public bool) (*ProtDevSrv, error) {
	if public {
		mfs, error := memfssrv.MakeMemFsPublic(fn, "protdevsrv")
		if error != nil {
			return nil, error
		}
		return MakeProtDevSrvMemFs(mfs, svci)
	} else {
		return MakeProtDevSrv(fn, svci)
	}
}

func MakeProtDevSrvPort(fn, port string, svci any) (*ProtDevSrv, error) {
	mfs, error := memfssrv.MakeMemFsPort(fn, ":"+port, "protdevsrv")
	if error != nil {
		db.DFatalf("protdevsrv.Run: %v\n", error)
	}
	return MakeProtDevSrvMemFs(mfs, svci)
}

func MakeProtDevSrvClnt(fn string, sc *sigmaclnt.SigmaClnt, svci any) (*ProtDevSrv, error) {
	mfs, error := memfssrv.MakeMemFsPortClnt(fn, ":0", sc)
	if error != nil {
		db.DFatalf("protdevsrv.Run: %v\n", error)
	}
	return MakeProtDevSrvMemFs(mfs, svci)
}

func MakeProtDevSrvMemFs(mfs *memfssrv.MemFs, svci any) (*ProtDevSrv, error) {
	psd := &ProtDevSrv{}
	psd.MemFs = mfs
	psd.mkService(svci)
	rd := mkRpcDev(psd)
	if err := sessdevsrv.MkSessDev(psd.MemFs, protdev.RPC, rd.mkRpcSession, nil); err != nil {
		return nil, err
	}
	if si, err := makeStatsDev(mfs); err != nil {
		return nil, err
	} else {
		psd.sti = si
	}
	return psd, nil
}

func (psd *ProtDevSrv) QueueLen() int64 {
	return psd.MemFs.QueueLen()
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
			mtype.NumIn() != 4 ||
			//mtype.In(1).Kind() != reflect.Ptr ||
			mtype.In(3).Kind() != reflect.Ptr ||
			mtype.NumOut() != 1 ||
			mtype.Out(0) != typeOfError {
			// the method is not suitable for a handler
			log.Printf("bad method: %v\n", mname)
		} else {
			// the method looks like a handler
			svc.methods[mname] = &method{methodt, mtype.In(2), mtype.In(3)}
		}
	}
	psd.svc = svc
}

func (psd *ProtDevSrv) RunServer() error {
	db.DPrintf(db.PROTDEVSRV, "Run %v\n", proc.GetProgram())
	psd.MemFs.Serve()
	psd.MemFs.Done()
	return nil
}
