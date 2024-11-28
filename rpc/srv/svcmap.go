package srv

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	db "sigmaos/debug"
)

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

type method struct {
	method    reflect.Method
	argType   reflect.Type
	replyType reflect.Type
}

type service struct {
	svc     reflect.Value
	typ     reflect.Type
	methods map[string]*method
}

type svcMap struct {
	sync.Mutex
	svc map[string]*service
}

func newSvcMap() *svcMap {
	return &svcMap{svc: make(map[string]*service)}
}

// Add a new RPC service to the svc map
func (svcmap *svcMap) RegisterService(svci any) {
	svcmap.Lock()
	defer svcmap.Unlock()

	svc := &service{}
	svc.typ = reflect.TypeOf(svci)
	svc.svc = reflect.ValueOf(svci)
	svc.methods = map[string]*method{}

	tname := structName(svci)
	db.DPrintf(db.SIGMASRV, "newRPCSrv %T %q\n", svci, tname)

	for m := 0; m < svc.typ.NumMethod(); m++ {
		methodt := svc.typ.Method(m)
		mtype := methodt.Type
		mname := methodt.Name

		if methodt.PkgPath != "" || // capitalized?
			mtype.NumIn() != 4 ||
			//mtype.In(1).Kind() != reflect.Ptr ||
			mtype.In(3).Kind() != reflect.Ptr ||
			mtype.NumOut() != 1 ||
			mtype.Out(0) != typeOfError {
			// the method is not suitable for a handler
			db.DPrintf(db.ALWAYS, "%v: bad method: %v\n", tname, mname)
		} else {
			// the method looks like a handler
			svc.methods[mname] = &method{methodt, mtype.In(2), mtype.In(3)}
		}
	}
	svcmap.svc[tname] = svc
}

func (svcmap *svcMap) lookup(tname string) (*service, error) {
	svcmap.Lock()
	defer svcmap.Unlock()

	svc, ok := svcmap.svc[tname]
	if !ok {
		db.DPrintf(db.ERROR, "Unknown tname %q %v\n%s", tname, svcmap, debug.Stack())
		return nil, fmt.Errorf("Unknown tname %q %v\n%s", tname, svcmap, debug.Stack())
	}
	return svc, nil
}

func structName(svci any) string {
	typ := reflect.TypeOf(svci)
	name := typ.String()
	dot := strings.LastIndex(name, ".")
	return name[dot+1:]
}
