package protdevsrv

import (
	"bytes"
	"encoding/gob"
	"log"
	"reflect"
	"strings"
	"time"

	// db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/ninep"
)

type Stream struct {
	svc      *service
	sts      *Stats
	inflight bool
	repl     []byte
}

func mkStream(sts *Stats, svc *service) (fs.File, *np.Err) {
	st := &Stream{}
	st.svc = svc
	st.sts = sts
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

	start := time.Now()
	rep = st.svc.dispatch(req.Method, req)
	t := time.Since(start).Microseconds()
	st.sts.stat(req.Method, t)

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
