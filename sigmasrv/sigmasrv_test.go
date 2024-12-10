package sigmasrv_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type Server struct {
}

func (s *Server) f() {
}

func g(svci any) {
	typ := reflect.TypeOf(svci)
	dot := strings.LastIndex(typ.String(), ".")
	svc := typ.String()[dot+1:]
	fmt.Printf("svc %q %s %q %q\n", typ.Name(), typ.String(), typ.PkgPath(), svc)
}

func TestServer(t *testing.T) {
	s := &Server{}
	g(s)
}
