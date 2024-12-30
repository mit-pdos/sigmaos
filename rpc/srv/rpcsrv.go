package srv

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type RPCSrv struct {
	svc *svcMap
	sti *rpc.StatInfo
}

func NewRPCSrv(svci any, si *rpc.StatInfo) *RPCSrv {
	rpcs := &RPCSrv{svc: newSvcMap(), sti: si}
	rpcs.RegisterService(svci)
	return rpcs
}

func (rpcs *RPCSrv) RegisterService(svci any) {
	rpcs.svc.RegisterService(svci)
}

func (rpcs *RPCSrv) WriteRead(ctx fs.CtxI, iov sessp.IoVec) (sessp.IoVec, *serr.Err) {
	var start time.Time
	if rpcs.sti != nil {
		start = time.Now()
	}
	req := rpcproto.Request{}
	if err := proto.Unmarshal(iov[0], &req); err != nil {
		return nil, serr.NewErrError(err)
	}
	var rerr *sp.Rerror
	iov, sr := rpcs.ServeRPC(ctx, req.Method, iov[1:])
	if sr != nil {
		rerr = sp.NewRerrorSerr(sr)
	} else {
		rerr = sp.NewRerror()
	}
	rep := &rpcproto.Reply{Err: rerr}
	b, err := proto.Marshal(rep)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	if rpcs.sti != nil {
		rpcs.sti.Stat(req.Method, time.Since(start).Microseconds())
	}
	iov = append(sessp.IoVec{b}, iov...)
	return iov, nil
}

func (rpcs *RPCSrv) ServeRPC(ctx fs.CtxI, m string, iov sessp.IoVec) (sessp.IoVec, *serr.Err) {
	dot := strings.LastIndex(m, ".")
	if dot <= 0 {
		return nil, serr.NewErrError(fmt.Errorf("Invalid method %q", m))
	}
	method := m[dot+1:]
	tname := m[:dot]
	db.DPrintf(db.SIGMASRV, "serveRPC svc %v name %v\n", tname, method)
	serv, r := rpcs.svc.lookup(tname)
	if r != nil {
		return nil, serr.NewErrError(r)
	}
	repmsg, err := serv.dispatch(ctx, m, iov)
	if err != nil {
		return nil, err
	}
	var iovrep sessp.IoVec
	blob := rpc.GetBlob(repmsg)
	if blob != nil {
		iovrep = blob.GetIoVec()
		blob.SetIoVec(nil)
	}
	b, r := proto.Marshal(repmsg)
	if r != nil {
		return nil, serr.NewErrError(r)
	}
	iovrep = append(sessp.IoVec{b}, iovrep...)
	return iovrep, nil
}

func (svc *service) dispatch(ctx fs.CtxI, methname string, iov sessp.IoVec) (proto.Message, *serr.Err) {
	dot := strings.LastIndex(methname, ".")
	name := methname[dot+1:]
	if method, ok := svc.methods[name]; ok {
		// prepare space into which to read the argument.
		// the Value's type will be a pointer to req.argsType.
		args := reflect.New(method.argType)
		reqmsg := args.Interface().(proto.Message)
		if err := proto.Unmarshal(iov[0], reqmsg); err != nil {
			return nil, serr.NewErrError(err)
		}
		blob := rpc.GetBlob(reqmsg)
		if blob != nil {
			blob.SetIoVec(iov[1:])
		}
		db.DPrintf(db.SIGMASRV, "dispatchproto %v %v %v\n", svc.svc, name, reqmsg)

		// allocate space for the reply.
		replyType := method.replyType
		replyType = replyType.Elem()
		replyv := reflect.New(replyType)
		repmsg := replyv.Interface().(proto.Message)

		// call the method.
		function := method.method.Func
		rv := function.Call([]reflect.Value{svc.svc, reflect.ValueOf(ctx), args.Elem(), replyv})

		errI := rv[0].Interface()
		if errI != nil { // The return value for the method if it is an error.
			err := errI.(error)
			var sr *serr.Err
			if errors.As(err, &sr) {
				return nil, sr
			}
			return nil, serr.NewErrError(err)
		}
		return repmsg, nil
	} else {
		choices := []string{}
		for k, _ := range svc.methods {
			choices = append(choices, k)
		}
		db.DPrintf(db.ALWAYS, "rpcDev.dispatch(): unknown method %v in %v; expecting one of %v\n",
			methname, name, choices)
		return nil, serr.NewErr(serr.TErrNotfound, methname)
	}
}
