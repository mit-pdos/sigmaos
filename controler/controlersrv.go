package controler

import (
	"container/list"
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"sync"

	"ulambda/common"
)

// XXX use Lambda.Net
type Lambda struct {
	Name  string
	Arg   []byte
	Reply *[]byte
}

type ControlerSrv struct {
	mu    sync.Mutex
	cond  *sync.Cond
	queue *list.List
	clnt  *rpc.Client
}

func mkControlerSrv() *ControlerSrv {
	s := &ControlerSrv{}
	s.queue = list.New()
	s.cond = sync.NewCond(&s.mu)
	go s.scheduler()
	return s
}

func (s *ControlerSrv) run(l *Lambda) {
	fmt.Printf("Run %v\n", l)
	var reply int
	args := common.LambdaNet{l.Name, l.Arg}
	err := s.clnt.Call("LambdaSrv.Run", args, &reply)
	if err != nil {
		log.Fatal("Call error:", err)
	}
	// l.Reply = &reply
}

func (s *ControlerSrv) scheduler() {
	s.mu.Lock()
	for {
		for e := s.queue.Front(); e != nil; e = e.Next() {
			s.queue.Remove(e)
			l := e.Value.(*Lambda)
			go s.run(l)

		}
		s.cond.Wait()
	}
}

type RegisterArgs struct {
	Name string
	Port int
}

func (s *ControlerSrv) Register(args *RegisterArgs, reply *int) error {
	fmt.Printf("register %v\n", args)
	c, err := rpc.Dial("tcp", ":"+strconv.Itoa(args.Port))
	if err != nil {
		log.Fatal("dialing:", err)
	}
	s.clnt = c
	*reply = 0
	return nil
}

type ForkArgs struct {
	Name string
	Arg  []byte
}

// func (s *ControlerSrv) Fork(args *common.LambdaNet, reply *int) error {
func (s *ControlerSrv) Fork(args *ForkArgs, reply *int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fmt.Printf("Fork %v\n", args)
	l := &Lambda{args.Name, args.Arg, new([]byte)}
	//l := &Lambda{}
	s.queue.PushBack(l)
	s.cond.Signal()
	*reply = 0
	return nil
}
