package chunksrv

import (
	"os"
	"path"
	"time"

	proto "sigmaos/chunksrv/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	CHUNKSZ = 1 * sp.MBYTE

	SEEK_DATA = 3
	SEEK_HOLE = 4
)

func Index(o int64) int { return int(o / CHUNKSZ) }
func Ckoff(i int) int64 { return int64(i * CHUNKSZ) }

type ChunkSrv struct {
	sc       *sigmaclnt.SigmaClnt
	kernelId string
}

func newChunkSrv(kernelId string, sc *sigmaclnt.SigmaClnt) *ChunkSrv {
	cksrv := &ChunkSrv{sc: sc, kernelId: kernelId}
	return cksrv
}

func Fetch(sc *sigmaclnt.SigmaClnt, pn string, req proto.FetchRequest, res *proto.FetchResponse) error {
	ckid := int(req.ChunkId)
	s := time.Now()
	sfd := 0
	db.DPrintf(db.CHUNKSRV, "Fetch: read %q ck %d", req.Prog, ckid)
	if err := fslib.RetryPaths(req.Path, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "sOpen %q/%v\n", pn, req.Prog)
		fd, err := sc.Open(pn+"/"+req.Prog, sp.OREAD)
		if err == nil {
			sfd = fd
			return nil
		}
		return err
	}); err != nil {
		return err
	}
	defer func() {
		sc.CloseFd(sfd)
		db.DPrintf(db.SPAWN_LAT, "[%v] Fetch chunk %d %v", req.Prog, ckid, time.Since(s))
	}()
	db.DPrintf(db.SPAWN_LAT, "[%v] Open %v %v", req.Prog, req.Path, time.Since(s))

	s1 := time.Now()
	b := make([]byte, CHUNKSZ)
	sz, err := sc.Pread(sfd, b, sp.Toffset(Ckoff(ckid)))
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "Fetch: read %q ck %d err %v", req.Prog, ckid, err)
		return err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Read ck %d %v", req.Prog, ckid, time.Since(s1))

	db.DPrintf(db.CHUNKSRV, "Fetch: write %q ck %d", pn, ckid)
	if err := WriteChunk(pn, Ckoff(ckid), b[0:sz]); err != nil {
		db.DPrintf(db.CHUNKSRV, "Fetch: writechunk %q %d err %v", pn, ckid, err)
		return err
	}
	res.Size = uint64(sz)
	db.DPrintf(db.SPAWN_LAT, "[%v] Fetch: write %d %v", req.Prog, ckid, time.Since(s))
	return nil
}

func (cksrv *ChunkSrv) Fetch(ctx fs.CtxI, req proto.FetchRequest, res *proto.FetchResponse) error {
	db.DPrintf(db.CHUNKSRV, "Fetch %v", req)
	pn := path.Join(sp.SIGMAHOME, "bin/user/realms", req.Realm, req.Prog)
	return Fetch(cksrv.sc, pn, req, res)
}

func ReadChunk(pn string, ck int, totsz sp.Tsize) (int64, bool) {
	db.DPrintf(db.CHUNKSRV, "readLocal %q %d sz %d\n", pn, ck, totsz)
	f, err := os.OpenFile(pn, os.O_RDONLY, 0777)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sz := int64(0)
	ok := false
	for off := int64(0); off < int64(totsz); {
		o1, err := f.Seek(off, SEEK_DATA)
		if err != nil {
			break
		}
		o2, err := f.Seek(o1, SEEK_HOLE)
		if err != nil {
			db.DFatalf("Seek hole %q %d err %v\n", pn, o2, err)
		}
		for o := o1; o < o2; o += CHUNKSZ {
			if o%CHUNKSZ != 0 {
				db.DFatalf("offset %d\n", o)
			}
			if o+CHUNKSZ <= o2 || o2 >= int64(totsz) { // a complete chunk?
				i := Index(o)
				if i == ck {
					db.DPrintf(db.CHUNKSRV, "readLocal: %q read chunk %d(%d)\n", pn, i, o)
					ok = true
					sz = CHUNKSZ
					if o2 >= int64(totsz) {
						sz = int64(totsz) - o1
					}
					break
				}
			}
		}
		off = o2
	}
	return sz, ok
}

func WriteChunk(pn string, off int64, b []byte) error {
	s := time.Now()
	ufd, err := os.OpenFile(pn, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	defer ufd.Close()
	if _, err := ufd.Seek(off, 0); err != nil {
		return err
	}
	nn, err := ufd.Write(b)
	if nn != len(b) {
		return err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] writeCache %v", pn, time.Since(s))
	return nil
}

func Run(kernelId string) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	cksrv := newChunkSrv(kernelId, sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(path.Join(sp.CHUNKD, sc.ProcEnv().GetKernelID()), sc, cksrv)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	ssrv.RunServer()
}
