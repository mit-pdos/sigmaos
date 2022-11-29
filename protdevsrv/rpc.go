package protdevsrv

import (
	"reflect"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	np "sigmaos/sigmap"
	rpcproto "sigmaos/protdevsrv/proto"
)

type rpcDev struct {
	pds *ProtDevSrv
}

func mkRpcDev(pds *ProtDevSrv) *rpcDev {
	return &rpcDev{pds}
}

type rpcSession struct {
	*inode.Inode
	pds *ProtDevSrv
}

func (rd *rpcDev) mkRpcSession(mfs *memfssrv.MemFs, sid np.Tsession) (fs.Inode, *np.Err) {
	rpc := &rpcSession{}
	rpc.pds = rd.pds
	rpc.Inode = mfs.MakeDevInode()
	return rpc, nil
}

// XXX wait on close before processing data?
func (rpc *rpcSession) WriteRead(ctx fs.CtxI, b []byte) ([]byte, *np.Err) {
	req := rpcproto.Request{}
	var rep *rpcproto.Reply
	if err := proto.Unmarshal(b, &req); err != nil {
		return nil, np.MkErrError(err)
	}

	db.DPrintf("PROTDEVSRV", "WriteRead req %v\n", req)

	ql := rpc.pds.QueueLen()
	start := time.Now()
	rep = rpc.pds.svc.dispatch(req.Method, &req)
	t := time.Since(start).Microseconds()
	rpc.pds.sti.Stat(req.Method, t, ql)

	b, err := proto.Marshal(rep)
	if err != nil {
		return nil, np.MkErrError(err)
	}
	return b, nil
}

func (svc *service) dispatch(methname string, req *rpcproto.Request) *rpcproto.Reply {
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

		db.DPrintf("PROTDEVSRV", "dispatchproto %v %v\n", name, reqmsg)

		// allocate space for the reply.
		replyType := method.replyType
		replyType = replyType.Elem()
		replyv := reflect.New(replyType)
		repmsg := replyv.Interface().(proto.Message)

		// call the method.
		function := method.method.Func
		rv := function.Call([]reflect.Value{svc.svc, args.Elem(), replyv})

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
