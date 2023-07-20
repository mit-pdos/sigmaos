package sigmasrv

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
// Many SigmaOS servers use SigmaSrv to create and run servers.  A server
// typically consists of a MemFS (an in-memory file system accessed
// through sigmap), one or more RPC end points, including an end point
// for leasesrv (to manage leases).  Sigmasrv creates the end-points
// in the memfs. Some servers don't use SigmaSrv and directly interact
// with SessSrv (e.g., ux and knamed/named).
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

type SigmaSrv struct {
	*memfssrv.MemFs
	sti  *protdev.StatInfo
	svc  *service
	lsrv *LeaseSrv
}

// Make a sigmasrv and memfs and publish srv at fn
func MakeSigmaSrv(fn string, svci any, uname sp.Tuname) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFs(fn, uname)
	if error != nil {
		db.DFatalf("MakeSigmaSrv %v err %v\n", fn, error)
	}
	// XXX pull "rpc" upto here
	return MakeSigmaSrvMemFs(mfs, "", svci)
}

func MakeSigmaSrvPublic(fn string, svci any, uname sp.Tuname, public bool) (*SigmaSrv, error) {
	if public {
		mfs, error := memfssrv.MakeMemFsPublic(fn, uname)
		if error != nil {
			return nil, error
		}
		return MakeSigmaSrvMemFs(mfs, "", svci)
	} else {
		return MakeSigmaSrv(fn, svci, uname)
	}
}

func MakeSigmaSrvPort(fn, port string, uname sp.Tuname, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFsPort(fn, ":"+port, uname)
	if error != nil {
		db.DFatalf("MakeSigmaSrvPort %v err %v\n", fn, error)
	}
	return MakeSigmaSrvMemFs(mfs, "", svci)
}

func MakeSigmaSrvClnt(fn string, sc *sigmaclnt.SigmaClnt, uname sp.Tuname, svci any) (*SigmaSrv, error) {
	mfs, error := memfssrv.MakeMemFsPortClnt(fn, ":0", sc)
	if error != nil {
		db.DFatalf("MakeSigmaSrvClnt %v err %v\n", fn, error)
	}
	return MakeRPCSrv(mfs, "", svci)
}

// Make a SigmaSrv with protdev at pn in mfs
func MakeSigmaSrvMemFs(mfs *memfssrv.MemFs, pn string, svci any) (*SigmaSrv, error) {
	ssrv, err := MakeRPCSrv(mfs, pn, svci)
	if err != nil {
		return nil, err
	}
	if err := ssrv.NewLeaseSrv(); err != nil {
		return nil, err
	}
	return ssrv, nil
}

// Make a SigmaSrv with protdev at pn in mfs (without a lease server)
func MakeRPCSrv(mfs *memfssrv.MemFs, pn string, svci any) (*SigmaSrv, error) {
	ssrv := &SigmaSrv{MemFs: mfs}
	if svci != nil {
		ssrv.mkService(svci)
		rd := mkRpcDev(ssrv)
		if err := sessdevsrv.MkSessDev(ssrv.MemFs, path.Join(pn, protdev.RPC), rd.mkRpcSession, nil); err != nil {
			return nil, err
		}
		if si, err := makeStatsDev(mfs, pn); err != nil {
			return nil, err
		} else {
			ssrv.sti = si
		}
	}
	return ssrv, nil
}

func (ssrv *SigmaSrv) NewLeaseSrv() error {
	db.DPrintf(db.PROTDEVSRV, "NewLeaseSrv\n")
	lsrv := newLeaseSrv(ssrv.MemFs)
	if _, err := ssrv.Create(sp.LEASESRV, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	_, err := MakeRPCSrv(ssrv.MemFs, sp.LEASESRV, lsrv)
	if err != nil {
		return err
	}
	return nil
}

func (ssrv *SigmaSrv) QueueLen() int64 {
	return ssrv.MemFs.QueueLen()
}

func (ssrv *SigmaSrv) mkService(svci any) {
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
	ssrv.svc = svc
}

func (ssrv *SigmaSrv) RunServer() error {
	db.DPrintf(db.PROTDEVSRV, "Run %v\n", proc.GetProgram())
	ssrv.MemFs.Serve()
	if ssrv.lsrv != nil {
		ssrv.lsrv.Stop()
	}
	ssrv.MemFs.Exit(proc.MakeStatus(proc.StatusEvicted))
	return nil
}
