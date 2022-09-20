package fslibsrv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/dir"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	"sigmaos/fssrv"
	"sigmaos/memfs"
	ps "sigmaos/protsrv"
)

// start a minimal server to, for example, connect the proxy too by hand
func TestServer(t *testing.T) {
	root := dir.MkRootDir(nil, memfs.MakeInode)
	ip, err := fidclnt.LocalIP()
	assert.Nil(t, err, "LocalIP")
	srv := fssrv.MakeFsServer(root, ip+fslib.NamedAddr(), nil, ps.MakeProtServer, nil, nil, nil)
	srv.Serve()
}
