package fslibsrv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/dir"
	"ulambda/fsclnt"
	"ulambda/fslib"
	fos "ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/memfs"
)

func TestServer(t *testing.T) {
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode, memfs.GenPath)
	ip, err := fsclnt.LocalIP()
	assert.Nil(t, err, "LocalIP")
	srv := fssrv.MakeFsServer(root, ip+fslib.NamedAddr(), nil, fos.MakeProtServer, nil, nil)
	srv.Serve()

}
