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

type Lambda struct {
	Name   string
	Arg    []byte
	Reply  *[]byte
	Id     int64
	doneCh chan error
}

type ControlerSrv struct {
	mu      sync.Mutex
	cond    *sync.Cond
	queue   *list.List
	clnt    *rpc.Client
	running map[int64]*Lambda
}

func mkControlerSrv() *ControlerSrv {
	s := &ControlerSrv{}
	s.queue = list.New()
	s.running = make(map[int64]*Lambda)
	s.cond = sync.NewCond(&s.mu)
	go s.scheduler()
	return s
}

func (s *ControlerSrv) run(l *Lambda) error {
	fmt.Printf("Run %v\n", l)
	var reply common.LambdaReply
	args := common.LambdaReq{l.Name, l.Arg, l.Id}
	err := s.clnt.Call("LambdaSrv.Run", args, &reply)
	if err != nil {
		log.Fatal("Call error:", err)
	}
	l.Reply = &reply.Reply
	return err
}

func (s *ControlerSrv) scheduler() {
	s.mu.Lock()
	for {
		for e := s.queue.Front(); e != nil; e = e.Next() {
			s.queue.Remove(e)
			l := e.Value.(*Lambda)
			s.running[l.Id] = l
			go func(l *Lambda) {
				err := s.run(l)
				l.doneCh <- err
			}(l)

		}
		s.cond.Wait()
	}
}

type RegisterReq struct {
	Name string
	Port int
}

func (s *ControlerSrv) Register(args *RegisterReq, reply *int) error {
	fmt.Printf("register %v\n", args)
	c, err := rpc.Dial("tcp", ":"+strconv.Itoa(args.Port))
	if err != nil {
		log.Fatal("dialing:", err)
	}
	s.clnt = c
	*reply = 0
	return nil
}

type ForkReq struct {
	Name string
	Arg  []byte
	Id   int64
}

func (s *ControlerSrv) Fork(args *ForkReq, reply *int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fmt.Printf("Fork %v\n", args)
	l := &Lambda{args.Name, args.Arg, nil, args.Id, make(chan error)}
	s.queue.PushBack(l)
	s.cond.Signal()
	*reply = 0
	return nil
}

type WaitReq struct {
	Name string
	Id   int64
}

type WaitReply struct {
	Reply []byte
}

func (s *ControlerSrv) Join(args *WaitReq, reply *WaitReply) error {
	s.mu.Lock()
	l, ok := s.running[args.Id]
	if ok {
		delete(s.running, args.Id)
	}
	s.mu.Unlock()
	if ok {
		fmt.Printf("Join: wait %v\n", l)
		err := <-l.doneCh
		fmt.Printf("Join: lambda finished id %v err %v %v\n", l.Id, err, l.Reply)
		reply.Reply = *l.Reply
		return err
	} else {
		log.Fatal("Join: unknown lambda %v\n", args.Id)
	}
	return nil
}
