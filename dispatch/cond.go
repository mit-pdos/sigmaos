package dispatch

import (
	"sync"
)

type MakeCondF func(sync.Locker) Cond

type Cond interface {
	Signal()
	Wait() //error
}

var MakeCond MakeCondF

func SetMakeCondF() {
	MakeCond = func(l sync.Locker) Cond {
		return sync.NewCond(l)
	}
}
