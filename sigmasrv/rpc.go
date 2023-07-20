package sigmasrv

import (
	"reflect"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	rpcproto "sigmaos/protdev/proto"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type rpcDev struct {
	ssrv *SigmaSrv
}

func mkRpcDev(ssrv *SigmaSrv) *rpcDev {
	return &rpcDev{ssrv}
}

type rpcSession struct {
	*inode.Inode
	ssrv *SigmaSrv
}

func (rd *rpcDev) mkRpcSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	rpc := &rpcSession{}
	rpc.ssrv = rd.ssrv
	rpc.Inode = mfs.MakeDevInode()
	return rpc, nil
}

// XXX wait on close before processing data?
func (rpc *rpcSession) WriteRead(ctx fs.CtxI, b []byte) ([]byte, *serr.Err) {
	req := rpcproto.Request{}
	var rep *rpcproto.Reply
	if err := proto.Unmarshal(b, &req); err != nil {
		return nil, serr.MkErrError(err)
	}

	db.DPrintf(db.PROTDEVSRV, "WriteRead req %v\n", req)

	start := time.Now()
	rep = rpc.ssrv.svc.dispatch(ctx, req.Method, &req)
	t := time.Since(start).Microseconds()
	rpc.ssrv.sti.Stat(req.Method, t)

	b, err := proto.Marshal(rep)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

func (svc *service) dispatch(ctx fs.CtxI, methname string, req *rpcproto.Request) *rpcproto.Reply {
	dot := strings.LastIndex(methname, ".")
	name := methname[dot+1:]
	if method, ok := svc.methods[name]; ok {
		// prepare space into which to read the argument.
		// the Value's type will be a pointer to req.argsType.
		args := reflect.New(method.argType)
		reqmsg := args.Interface().(proto.Message)
		if err := proto.Unmarshal(req.Args, reqmsg); err != nil {
			r := &rpcproto.Reply{}
			r.Error = err.Error()
			return r
		}

		db.DPrintf(db.PROTDEVSRV, "dispatchproto %v %v %v\n", svc.svc, name, reqmsg)

		// allocate space for the reply.
		replyType := method.replyType
		replyType = replyType.Elem()
		replyv := reflect.New(replyType)
		repmsg := replyv.Interface().(proto.Message)

		// call the method.
		function := method.method.Func
		rv := function.Call([]reflect.Value{svc.svc, reflect.ValueOf(ctx), args.Elem(), replyv})

		// The return value for the method is an error.
		errI := rv[0].Interface()
		errmsg := ""
		if errI != nil {
			errmsg = errI.(error).Error()
		}

		b, err := proto.Marshal(repmsg)
		if err != nil {
			errmsg = err.Error()
		}
		r := &rpcproto.Reply{}
		r.Res = b
		r.Error = errmsg
		return r
	} else {
		choices := []string{}
		for k, _ := range svc.methods {
			choices = append(choices, k)
		}
		db.DPrintf(db.ALWAYS, "rpcDev.dispatch(): unknown method %v in %v; expecting one of %v\n",
			methname, req.Method, choices)
		r := &rpcproto.Reply{}
		r.Error = "unknown method"
		return r
	}
}
