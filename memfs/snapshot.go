package memfs

import (
	"encoding/json"
	"log"
	"runtime/debug"

	"ulambda/fs"
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

func restoreFile(fn fs.RestoreF, b []byte) fs.FsObj {
	fs := &FileSnapshot{}
	err := json.Unmarshal(b, fs)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in restoreFile: %v", err)
	}
	f := &File{}
	f.FsObj = inode.RestoreInode(fn, fs.InodeSnap)
	f.data = fs.Data
	return f
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

func restoreSymlink(fn fs.RestoreF, b []byte) fs.FsObj {
	fs := &SymlinkSnapshot{}
	err := json.Unmarshal(b, fs)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in restoreSymlink: %v", err)
	}
	f := &Symlink{}
	f.FsObj = inode.RestoreInode(fn, fs.InodeSnap)
	f.target = fs.Target
	return f
}

func encode(o interface{}) []byte {
	b, err := json.Marshal(o)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL Error snapshot encoding memfs obj: %v", err)
	}
	return b
}
