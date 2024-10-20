package mr

import (
	"sync"

	"github.com/fmstephe/unsafeutil"
)

type kvmap struct {
	mu     sync.Mutex
	mincap int
	maxcap int
	kvs    map[string]*values
}

type values struct {
	mu sync.Mutex
	k  string
	vs []string
}

func newKvmap(mincap, maxcap int) *kvmap {
	return &kvmap{
		mincap: mincap,
		maxcap: maxcap,
		kvs:    make(map[string]*values),
	}
}

func (kvm kvmap) lookup(key []byte) *values {
	kvm.mu.Lock()
	defer kvm.mu.Unlock()

	k := unsafeutil.BytesToString(key)
	if e, ok := kvm.kvs[k]; ok {
		return e
	}
	k = string(key)
	v := &values{
		k:  k,
		vs: make([]string, 0, kvm.mincap),
	}
	kvm.kvs[k] = v
	return v
}

func (kvm *kvmap) combine(key []byte, value string, combinef ReduceT) error {
	e := kvm.lookup(key)
	if err := e.combine(value, combinef, kvm.maxcap); err != nil {
		return err
	}
	return nil
}

func (kvm *kvmap) emit(combinef ReduceT, emit EmitT) error {
	kvm.mu.Lock()
	defer kvm.mu.Unlock()

	for k, e := range kvm.kvs {
		if err := combinef(k, e.vs, emit); err != nil {
			return err
		}
	}
	return nil
}

func (dst *kvmap) merge(src *kvmap, combinef ReduceT) {
	dst.mu.Lock()
	defer dst.mu.Unlock()
	src.mu.Lock()
	defer src.mu.Unlock()

	for k, e := range src.kvs {
		k0 := unsafeutil.StringToBytes(k)
		d := dst.lookup(k0)
		for _, v := range e.vs {
			d.combine(v, combinef, dst.maxcap)
		}
	}
}

func (e *values) combine(value string, combinef ReduceT, maxcap int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.vs)+1 >= maxcap {
		e.vs = append(e.vs, value)
		if err := combinef(e.k, e.vs, func(key []byte, val string) error {
			e.vs = e.vs[:1]
			e.vs[0] = val
			return nil
		}); err != nil {
			return err
		}
	} else {
		e.vs = append(e.vs, value)
	}
	return nil
}
