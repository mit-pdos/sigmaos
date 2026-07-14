package fsux

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/proxy/ux/proto"
	rpcproto "sigmaos/rpc/proto"
)

type UXRpcAPI struct {
	mu   sync.Mutex
	root string
}

func newUXRpcAPI(root string) *UXRpcAPI {
	return &UXRpcAPI{
		root: root,
	}
}

func (ra *UXRpcAPI) fullPath(path string) string {
	return filepath.Join(ra.root, path)
}

func (ra *UXRpcAPI) GetFile(ctx fs.CtxI, req proto.UXReq, rep *proto.UXRep) error {
	db.DPrintf(db.UX, "GetFile RPC: path:%v off:%v cnt:%v", req.Path, req.Offset, req.Count)

	// Construct full path
	fullPath := ra.fullPath(req.Path)

	// Open file
	file, err := os.Open(fullPath)
	if err != nil {
		db.DPrintf(db.UX_ERR, "Err Open: %v", err)
		db.DPrintf(db.ERROR, "Err Open: %v", err)
		return err
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		db.DPrintf(db.UX_ERR, "Err Stat: %v", err)
		db.DPrintf(db.ERROR, "Err Stat: %v", err)
		return err
	}

	if req.Count > 0 {
		// Chunked read: [offset, offset+count), truncated at EOF. A range
		// starting at or past EOF returns an empty blob, not an error.
		nbyte := uint64(0)
		if sz := uint64(info.Size()); req.Offset < sz {
			nbyte = min(req.Count, sz-req.Offset)
		}
		rep.Blob = &rpcproto.Blob{
			Iov: [][]byte{make([]byte, nbyte)},
		}
		if nbyte > 0 {
			n, err := file.ReadAt(rep.Blob.Iov[0], int64(req.Offset))
			if uint64(n) != nbyte {
				db.DPrintf(db.UX_ERR, "Err ReadAt: n %v nbyte %v err %v", n, nbyte, err)
				db.DPrintf(db.ERROR, "Err ReadAt: n %v nbyte %v err %v", n, nbyte, err)
				return err
			}
		}
		db.DPrintf(db.UX, "GetFile RPC success: path:%v off:%v len:%v", req.Path, req.Offset, len(rep.Blob.Iov[0]))
		rep.OK = true
		return nil
	}

	nbyte := int(info.Size())
	// Set up the reply IOVec
	rep.Blob = &rpcproto.Blob{
		Iov: [][]byte{make([]byte, nbyte)},
	}

	n, err := io.ReadAtLeast(file, rep.Blob.Iov[0], nbyte)
	if n != nbyte || err != nil {
		db.DPrintf(db.UX_ERR, "Err Read: %v", err)
		db.DPrintf(db.ERROR, "Err Read: %v", err)
		return err
	}

	db.DPrintf(db.UX, "GetFile RPC success: path:%v len:%v", req.Path, len(rep.Blob.Iov[0]))
	rep.OK = true
	return nil
}

func (ra *UXRpcAPI) PutFile(ctx fs.CtxI, req proto.UXReq, rep *proto.UXRep) error {
	db.DPrintf(db.UX, "PutFile RPC: path:%v off:%v len:%v", req.Path, req.Offset, len(req.Blob.Iov[0]))

	// Construct full path
	fullPath := ra.fullPath(req.Path)

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		db.DPrintf(db.UX_ERR, "Err MkdirAll: %v", err)
		db.DPrintf(db.ERROR, "Err MkdirAll: %v", err)
		return err
	}

	if req.Offset > 0 {
		// Chunked write at a byte offset. Chunked writers write
		// sequentially, so the file was created (and truncated) by the
		// offset-0 chunk.
		file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			db.DPrintf(db.UX_ERR, "Err OpenFile: %v", err)
			db.DPrintf(db.ERROR, "Err OpenFile: %v", err)
			return err
		}
		defer file.Close()
		if _, err := file.WriteAt(req.Blob.Iov[0], int64(req.Offset)); err != nil {
			db.DPrintf(db.UX_ERR, "Err WriteAt: %v", err)
			db.DPrintf(db.ERROR, "Err WriteAt: %v", err)
			return err
		}
		db.DPrintf(db.UX, "PutFile RPC success: path:%v off:%v len:%v", req.Path, req.Offset, len(req.Blob.Iov[0]))
		rep.OK = true
		return nil
	}

	// Write file (offset 0: create/truncate)
	if err := os.WriteFile(fullPath, req.Blob.Iov[0], 0644); err != nil {
		db.DPrintf(db.UX_ERR, "Err WriteFile: %v", err)
		db.DPrintf(db.ERROR, "Err WriteFile: %v", err)
		return err
	}

	db.DPrintf(db.UX, "PutFile RPC success: path:%v len:%v", req.Path, len(req.Blob.Iov[0]))
	rep.OK = true
	return nil
}
