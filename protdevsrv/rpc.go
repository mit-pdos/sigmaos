package protdevsrv

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	np "sigmaos/ninep"
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
	req := &Request{}
	var rep *Reply

	// Read request
	ab := bytes.NewBuffer(b)
	ad := gob.NewDecoder(ab)
	if err := ad.Decode(req); err != nil {
		return nil, np.MkErrError(err)
	}
	db.DPrintf("PROTDEVSRV", "WriteRead req %v\n", req)
	ql := rpc.pds.QueueLen()
	start := time.Now()
	if req.Protobuf {
		rep = rpc.pds.svc.dispatchproto(req.Method, req)
	} else {
		rep = rpc.pds.svc.dispatch(req.Method, req)
	}
	t := time.Since(start).Microseconds()
	rpc.pds.sti.Stat(req.Method, t, ql)

	rb := new(bytes.Buffer)
	re := gob.NewEncoder(rb)
	if err := re.Encode(&rep); err != nil {
		return nil, np.MkErrError(err)
	}
	db.DPrintf("PROTDEVSRV", "Done writeread")
	return rb.Bytes(), nil
}

func (svc *service) dispatch(methname string, req *Request) *Reply {
	dot := strings.LastIndex(methname, ".")
	name := methname[dot+1:]
	if method, ok := svc.methods[name]; ok {
		// prepare space into which to read the argument.
		// the Value's type will be a pointer to req.argsType.
		args := reflect.New(method.argType)

		// decode the argument
		ab := bytes.NewBuffer(req.Args)
		ad := gob.NewDecoder(ab)
		if err := ad.Decode(args.Interface()); err != nil {
			return &Reply{nil, err.Error()}
		}
		// db.DPrintf("PROTDEVSRV", "dispatch %v\n")

		// allocate space for the reply.
		replyType := method.replyType
		replyType = replyType.Elem()
		replyv := reflect.New(replyType)

		// call the method.
		function := method.method.Func
		rv := function.Call([]reflect.Value{svc.svc, args.Elem(), replyv})

		// The return value for the method is an error.
		errI := rv[0].Interface()
		errmsg := ""
		if errI != nil {
			errmsg = errI.(error).Error()
		}

		// encode the reply.
		rb := new(bytes.Buffer)
		re := gob.NewEncoder(rb)
		re.EncodeValue(replyv)

		return &Reply{rb.Bytes(), errmsg}
	} else {
		choices := []string{}
		for k, _ := range svc.methods {
			choices = append(choices, k)
		}
		db.DPrintf(db.ALWAYS, "rpcDev.dispatch(): unknown method %v in %v; expecting one of %v\n",
			methname, req.Method, choices)
		return &Reply{nil, "uknown method"}
	}
}

func (svc *service) dispatchproto(methname string, req *Request) *Reply {
	dot := strings.LastIndex(methname, ".")
	name := methname[dot+1:]
	if method, ok := svc.methods[name]; ok {
		// prepare space into which to read the argument.
		// the Value's type will be a pointer to req.argsType.
		args := reflect.New(method.argType)
		reqmsg := args.Interface().(proto.Message)
		if err := proto.Unmarshal(req.Args, reqmsg); err != nil {
			return &Reply{nil, err.Error()}
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

		return &Reply{b, errmsg}
	} else {
		choices := []string{}
		for k, _ := range svc.methods {
			choices = append(choices, k)
		}
		db.DPrintf(db.ALWAYS, "rpcDev.dispatch(): unknown method %v in %v; expecting one of %v\n",
			methname, req.Method, choices)
		return &Reply{nil, "uknown method"}
	}
}
