package rpcsrv

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
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

func (rpcs *RPCSrv) WriteRead(ctx fs.CtxI, arg []byte) ([]byte, *serr.Err) {
	start := time.Now()
	req := rpcproto.Request{}
	if err := proto.Unmarshal(arg, &req); err != nil {
		return nil, serr.NewErrError(err)
	}
	var rerr *sp.Rerror
	b, sr := rpcs.ServeRPC(ctx, req.Method, req.Args)
	if sr != nil {
		rerr = sp.NewRerrorSerr(sr)
	} else {
		rerr = sp.NewRerror()
	}
	rep := &rpcproto.Reply{Res: b, Err: rerr}
	b, err := proto.Marshal(rep)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	rpcs.sti.Stat(req.Method, time.Since(start).Microseconds())
	return b, nil
}

func (rpcs *RPCSrv) ServeRPC(ctx fs.CtxI, m string, b []byte) ([]byte, *serr.Err) {
	dot := strings.LastIndex(m, ".")
	method := m[dot+1:]
	tname := m[:dot]
	db.DPrintf(db.SIGMASRV, "serveRPC svc %v name %v\n", tname, method)
	repmsg, err := rpcs.svc.lookup(tname).dispatch(ctx, m, b)
	if err != nil {
		return nil, err
	}
	b, r := proto.Marshal(repmsg)
	if r != nil {
		return nil, serr.NewErrError(r)
	}
	return b, nil

}

func (svc *service) dispatch(ctx fs.CtxI, methname string, req []byte) (proto.Message, *serr.Err) {
	dot := strings.LastIndex(methname, ".")
	name := methname[dot+1:]
	if method, ok := svc.methods[name]; ok {
		// prepare space into which to read the argument.
		// the Value's type will be a pointer to req.argsType.
		args := reflect.New(method.argType)
		reqmsg := args.Interface().(proto.Message)
		if err := proto.Unmarshal(req, reqmsg); err != nil {
			return nil, serr.NewErrError(err)
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
		if errI != nil { // The return value for the method is an error.
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
