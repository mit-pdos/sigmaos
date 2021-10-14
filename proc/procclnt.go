package proc

type GenericProc interface {
	GetProc() *Proc
	String() string
}

type ProcClnt interface {
	Spawn(GenericProc) error
	SpawnNew(GenericProc) error
	WaitStart(string) error
	WaitStartNew(string) error
	WaitExit(string) (string, error)
	WaitExitNew(string) (string, error)
	WaitEvict(string) error
	Started(string) error
	StartedNew(string) error
	Exited(string, string) error
	ExitedNew(string, string) error
	Evict(string) error
}
