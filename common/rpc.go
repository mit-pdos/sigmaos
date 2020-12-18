package common

type LambdaReq struct {
	Name string
	Arg  []byte
	Id   int64
}

type LambdaReply struct {
	Reply []byte
}
