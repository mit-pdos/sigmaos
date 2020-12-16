package lambda

type LambdaF func(*Lambda) error

type Lambda struct {
	c     *Controler
	f     LambdaF
	Args  interface{}
	Reply interface{}
}

func (l *Lambda) Fork(f LambdaF, args interface{}, reply interface{}) *Lambda {
	return l.c.Fork(f, args, reply)
}

func (l *Lambda) Join() error {
	l.f(l)
	return nil
}
