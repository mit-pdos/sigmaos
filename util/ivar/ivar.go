package ivar

import "sync"

type IVar[T any] struct {
	ready chan struct{}
	val   T
	once  sync.Once
}

func NewIVar[T any]() IVar[T] {
	return IVar[T]{
		ready: make(chan struct{}),
	}
}

func (i *IVar[T]) Fill(v T) {
	i.once.Do(func() {
		i.val = v
		close(i.ready)
	})
}

func (i *IVar[T]) Read() T {
	<-i.ready
	return i.val
}
