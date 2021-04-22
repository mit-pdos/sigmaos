package kv

type Tstatus int

const (
	COMMIT Tstatus = 0
	ABORT  Tstatus = 1
	CRASH  Tstatus = 2
)

func (s Tstatus) String() string {
	switch s {
	case COMMIT:
		return "COMMIT"
	case ABORT:
		return "ABORT"
	default:
		return "CRASH"
	}
}
