package memfs

import (
	"encoding/json"
	"log"
	"runtime/debug"

	"ulambda/inode"
)

type FileSnapshot struct {
	InodeSnap []byte
	Data      []byte
}

func makeFileSnapshot(f *File) []byte {
	fs := &FileSnapshot{}
	fs.InodeSnap = f.FsObj.(*inode.Inode).Snapshot()
	fs.Data = f.data
	return encode(fs)
}

type SymlinkSnapshot struct {
	InodeSnap []byte
	Target    []byte
}

func makeSymlinkSnapshot(s *Symlink) []byte {
	fs := &SymlinkSnapshot{}
	fs.InodeSnap = s.FsObj.(*inode.Inode).Snapshot()
	fs.Target = s.target
	return encode(fs)
}

func encode(o interface{}) []byte {
	b, err := json.Marshal(o)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL Error snapshot encoding memfs obj: %v", err)
	}
	return b
}
