package proc

type GenericProc interface {
	GetProc() *Proc
	String() string
}

type ProcClnt interface {
	Spawn(GenericProc) error
	WaitStart(string) error
	WaitExit(string) (string, error)
	WaitEvict(string) error
	Started(string) error
	Exited(string, string) error
	Evict(string) error
	ChildDir(string) string
}
