package protdevsrv

import (
	"bytes"
	"encoding/gob"
	"log"
	"reflect"
	"strings"
	"sync"

	// db "sigmaos/debug"
	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/ninep"
	"sigmaos/sesscond"
)

type Stream struct {
	sync.Mutex
	svc      *service
	sct      *sesscond.SessCondTable
	inflight bool
	repl     []byte
}

func mkStream(ctx fs.CtxI, svc *service) (fs.File, *np.Err) {
	st := &Stream{}
	st.svc = svc
	st.sct = ctx.SessCondTable()
	return st, nil
}

// XXX wait on close before processing data?
func (st *Stream) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	req := &Request{}
	var rep *Reply

	// Read request
	ab := bytes.NewBuffer(b)
	ad := gob.NewDecoder(ab)
	if err := ad.Decode(req); err != nil {
		return 0, np.MkErrError(err)
	}

	// Lock the stream.
	st.Lock()
	defer st.Unlock()

	// Sanity-check that there aren't concurrent RPCs on a single stream. These
	// should (currently) be synchronized on the client side.
	if st.inflight {
		db.DFatalf("Tried to perform an RPC on an already-inflight stream.")
	}

	// Mark that an RPC is in-flight.
	st.inflight = true

	// Create a sesscond to allow concurrent RPCs.
	cond := st.sct.MakeSessCond(&st.Mutex)

	// Dispatch RPC in a separate thread & store reply.
	go func() {
		rep = st.svc.dispatch(req.Method, req)
		// Wake up the writer thread.
		st.Lock()
		defer st.Unlock()

		// Mark that the in-flight RPC has terminated..
		st.inflight = false
		cond.Signal()
	}()

	// Wait for the RPC to complete. This allows other sessions to make
	// concurrent  RPCs to this server.
	for st.inflight {
		cond.Wait(ctx.SessionId())
	}
	st.sct.FreeSessCond(cond)

	rb := new(bytes.Buffer)
	re := gob.NewEncoder(rb)
	if err := re.Encode(&rep); err != nil {
		return 0, np.MkErrError(err)
	}
	st.repl = rb.Bytes()
	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (st *Stream) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}
	if st.repl == nil {
		return nil, nil
	}
	if np.Tsize(len(st.repl)) > cnt {
		np.MkErr(np.TErrInval, "too large")
	}
	return st.repl, nil
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
		log.Printf("dispatch(): unknown method %v in %v; expecting one of %v\n",
			methname, req.Method, choices)
		return &Reply{nil, "uknown method"}
	}
}
