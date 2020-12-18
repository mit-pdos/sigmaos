package lambda

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"reflect"
	"strconv"
)

type Lambda struct {
	c      *Controler
	name   string
	args   interface{}
	reply  interface{}
	id     int64
	doneCh chan error
}

// Create a root lambda
func (c *Controler) Fork(name string, args interface{}) *Lambda {
	id := rand.Int63()
	err := c.ForkLambda(name, args, id)
	if err == nil {
		l := &Lambda{c, name, args, nil, id, make(chan error)}
		return l
	} else {
		return nil
	}
}

// Wait for lambda to run. If it returns Reply must have been filled in.
func (l *Lambda) Join(reply interface{}) error {
	repbytes, err := l.c.JoinLambda(l.name, l.id)
	if err == nil {
		rb := bytes.NewBuffer(repbytes)
		rd := gob.NewDecoder(rb)
		if err := rd.Decode(reply); err != nil {
			log.Fatalf("Join: decode error: %v\n", err)
		}
	}
	return err
}

type Controler struct {
	clnt *rpc.Client
	srv  *LambdaSrv
	port int
}

// Make a persistent connection with controler
func ConnectToControler() *Controler {
	serverAddress := "localhost"
	client, err := rpc.Dial("tcp", serverAddress+":1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}

	// Start lambda server
	port := 2222
	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	c := &Controler{client, &LambdaSrv{make(map[string]*LambdaReceiver)}, port}
	rpc.Register(c.srv)
	go c.RunSrv(l)

	return c
}

func (c *Controler) RunSrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}
		go rpc.ServeConn(conn)
	}
}

type LambdaReceiver struct {
	name    string
	rcvr    reflect.Value
	typ     reflect.Type
	methods map[string]reflect.Method
}

func makeLambdaReceiver(l interface{}) *LambdaReceiver {
	lr := &LambdaReceiver{}
	lr.typ = reflect.TypeOf(l)
	lr.rcvr = reflect.ValueOf(l)
	lr.name = reflect.Indirect(lr.rcvr).Type().Name()
	lr.methods = map[string]reflect.Method{}

	for m := 0; m < lr.typ.NumMethod(); m++ {
		method := lr.typ.Method(m)
		mtype := method.Type
		mname := method.Name

		//fmt.Printf("%v ni %v 1k %v 2k %v no %v\n", mname, mtype.NumIn(),
		//	mtype.In(1).Kind(), mtype.In(2).Kind(), mtype.NumOut())

		if mtype.NumIn() != 3 ||
			//mtype.In(1).Kind() != reflect.Ptr ||
			mtype.In(2).Kind() != reflect.Ptr ||
			mtype.NumOut() != 1 {
			// the method is not suitable for a handler
			fmt.Printf("bad method: %v\n", mname)
		} else {
			// fmt.Printf("method: %v\n", mname)
			// the method looks like a handler
			lr.methods[mname] = method
		}
	}
	return lr
}

func (lr *LambdaReceiver) dispatch(methodn string, args []byte) ([]byte, bool) {
	if method, ok := lr.methods[methodn]; ok {
		// space for argument and decode
		a := reflect.New(method.Type.In(1))
		ab := bytes.NewBuffer(args)
		ad := gob.NewDecoder(ab)
		if err := ad.Decode(a.Interface()); err != nil {
			log.Fatal("Run: decode failure: ", err)
		}

		// space for reply
		replyType := method.Type.In(2)
		replyType = replyType.Elem() // dereference *
		replyVal := reflect.New(replyType)

		// call lambda
		function := method.Func
		function.Call([]reflect.Value{lr.rcvr, a.Elem(), replyVal})

		// encode the reply.
		rb := new(bytes.Buffer)
		re := gob.NewEncoder(rb)
		re.EncodeValue(replyVal)

		return rb.Bytes(), true

	} else {
		log.Fatal("Dispatch: unknown method: ", methodn)
	}
	return nil, false
}

func (c *Controler) RegisterLambda(l interface{}) {
	lr := makeLambdaReceiver(l)
	c.srv.lambdas[lr.name] = lr
	c.RegisterLambdaName(lr.name)
}
