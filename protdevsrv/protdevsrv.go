package protdevsrv

import (
	"log"
	"path"
	"reflect"

	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/protdev"
	"sigmaos/sessdevsrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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
	sti  *protdev.StatInfo
	svc  *service
	lsrv *LeaseSrv
}

// Make a protdevsrv and memfs and publish srv at fn
func MakeProtDevSrv(fn string, svci any, uname sp.Tuname) (*ProtDevSrv, error) {
	mfs, error := memfssrv.MakeMemFs(fn, uname)
	if error != nil {
		db.DFatalf("MakeProtDevSrv %v err %v\n", fn, error)
	}
	// XXX pull "rpc" upto here
	return MakeProtDevSrvMemFs(mfs, "", svci)
}

func MakeProtDevSrvPublic(fn string, svci any, uname sp.Tuname, public bool) (*ProtDevSrv, error) {
	if public {
		mfs, error := memfssrv.MakeMemFsPublic(fn, uname)
		if error != nil {
			return nil, error
		}
		return MakeProtDevSrvMemFs(mfs, "", svci)
	} else {
		return MakeProtDevSrv(fn, svci, uname)
	}
}

func MakeProtDevSrvPort(fn, port string, uname sp.Tuname, svci any) (*ProtDevSrv, error) {
	mfs, error := memfssrv.MakeMemFsPort(fn, ":"+port, uname)
	if error != nil {
		db.DFatalf("MakeProtDevSrvPort %v err %v\n", fn, error)
	}
	return MakeProtDevSrvMemFs(mfs, "", svci)
}

func MakeProtDevSrvClnt(fn string, sc *sigmaclnt.SigmaClnt, uname sp.Tuname, svci any) (*ProtDevSrv, error) {
	mfs, error := memfssrv.MakeMemFsPortClnt(fn, ":0", sc)
	if error != nil {
		db.DFatalf("MakeProtDevSrvClnt %v err %v\n", fn, error)
	}
	return MakeRPCSrv(mfs, "", svci)
}

// Make a ProtDevSrv with protdev at pn in mfs
func MakeProtDevSrvMemFs(mfs *memfssrv.MemFs, pn string, svci any) (*ProtDevSrv, error) {
	psd, err := MakeRPCSrv(mfs, pn, svci)
	if err != nil {
		return nil, err
	}
	if err := psd.NewLeaseSrv(); err != nil {
		return nil, err
	}
	return psd, nil
}

// Make a ProtDevSrv with protdev at pn in mfs (without a lease server)
func MakeRPCSrv(mfs *memfssrv.MemFs, pn string, svci any) (*ProtDevSrv, error) {
	psd := &ProtDevSrv{MemFs: mfs}
	if svci != nil {
		psd.mkService(svci)
		rd := mkRpcDev(psd)
		if err := sessdevsrv.MkSessDev(psd.MemFs, path.Join(pn, protdev.RPC), rd.mkRpcSession, nil); err != nil {
			return nil, err
		}
		if si, err := makeStatsDev(mfs, pn); err != nil {
			return nil, err
		} else {
			psd.sti = si
		}
	}
	return psd, nil
}

func (psd *ProtDevSrv) NewLeaseSrv() error {
	db.DPrintf(db.PROTDEVSRV, "NewLeaseSrv\n")
	lsrv := newLeaseSrv(psd.MemFs)
	if _, err := psd.Create(sp.LEASESRV, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	_, err := MakeRPCSrv(psd.MemFs, sp.LEASESRV, lsrv)
	if err != nil {
		return err
	}
	return nil
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
	if psd.lsrv != nil {
		psd.lsrv.Stop()
	}
	psd.MemFs.Exit(proc.MakeStatus(proc.StatusEvicted))
	return nil
}
