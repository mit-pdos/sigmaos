package mr

import (
	"github.com/fmstephe/unsafeutil"
)

type kvmap struct {
	mincap int
	maxcap int
	kvs    map[string]*values
}

type values struct {
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

func (e *values) combine(value string, combinef ReduceT, maxcap int) error {
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
