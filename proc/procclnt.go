package proc

type GenericProc interface {
	GetProc() *Proc
	String() string
}

type ProcClnt interface {
	Spawn(GenericProc) error
	WaitStart(string) error
	WaitExit(string) error
	WaitEvict(string) error
	Started(string) error
	Exited(string) error
	Evict(string) error
}
