package memfs

import (
	"encoding/json"
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/file"
	"ulambda/fs"
	"ulambda/inode"
)

type FileSnapshot struct {
	InodeSnap []byte
	FileSnap  []byte
}

func makeFileSnapshot(f *File) []byte {
	fs := &FileSnapshot{}
	fs.InodeSnap = f.Inode.Snapshot(nil)
	fs.FileSnap = f.File.Snapshot()
	return encode(fs)
}

func restoreFile(fn fs.RestoreF, b []byte) fs.Inode {
	fs := &FileSnapshot{}
	err := json.Unmarshal(b, fs)
	if err != nil {
		db.DFatalf("error unmarshal file in restoreFile: %v", err)
	}
	f := &File{}
	f.Inode = inode.RestoreInode(fn, fs.InodeSnap)
	f.File = file.RestoreFile(fs.FileSnap)
	return f
}

func encode(o interface{}) []byte {
	b, err := json.Marshal(o)
	if err != nil {
		debug.PrintStack()
		db.DFatalf("Error snapshot encoding memfs obj: %v", err)
	}
	return b
}
