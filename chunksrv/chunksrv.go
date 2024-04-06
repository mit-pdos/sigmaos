package chunksrv

import (
	"os"
	"path"
	"strings"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	proto "sigmaos/chunksrv/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/keys"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	CHUNKSZ = 1 * sp.MBYTE

	SEEK_DATA = 3
	SEEK_HOLE = 4

	ROOT = sp.SIGMAHOME + "/all-realm-bin"
)

func Index(o int64) int { return int(o / CHUNKSZ) }
func Ckoff(i int) int64 { return int64(i * CHUNKSZ) }

func BinPath(realm sp.Trealm, prog string) string {
	return path.Join(ROOT, realm.String(), prog)
}

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}

type ChunkSrv struct {
	sc       *sigmaclnt.SigmaClnt
	kernelId string
}

func newChunkSrv(kernelId string, sc *sigmaclnt.SigmaClnt) *ChunkSrv {
	cksrv := &ChunkSrv{sc: sc, kernelId: kernelId}
	return cksrv
}

func (cksrv *ChunkSrv) Fetch(ctx fs.CtxI, req proto.FetchChunkRequest, res *proto.FetchChunkResponse) error {
	db.DPrintf(db.CHUNKSRV, "Fetch %v", req)
	return nil
}

// XXX hack; how to handle ~local?
func downloadPaths(paths []string, kernelId string) []string {
	for i, p := range paths {
		if strings.HasPrefix(p, sp.UX) {
			paths[i] = strings.Replace(p, "~local", kernelId, 1)
		}
	}
	return paths
}

func Lookup(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (*sp.Stat, error) {
	db.DPrintf(db.CHUNKSRV, "Lookup %q %v\n", prog, paths)

	var st *sp.Stat
	err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "Stat %q/%q\n", pn, prog)
		sst, err := sc.Stat(pn + "/" + prog)
		if err == nil {
			sst.Dev = uint32(i)
			st = sst
			return nil
		}
		return err
	})
	return st, err
}

func Open(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (int, error) {
	sfd := -1
	if err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "sOpen %q/%v\n", pn, prog)
		fd, err := sc.Open(pn+"/"+prog, sp.OREAD)
		if err == nil {
			sfd = fd
			return nil
		}
		return err
	}); err != nil {
		return sfd, err
	}
	return sfd, nil
}

func FetchOrigin(sc *sigmaclnt.SigmaClnt, realm sp.Trealm, fd int, prog string, ckid int, b []byte) (sp.Tsize, error) {
	sz, err := sc.Pread(fd, b, sp.Toffset(Ckoff(ckid)))
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "Fetch: read %q ck %d err %v", prog, ckid, err)
		return 0, err
	}
	return sz, nil
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
	return nil
}

func Run(kernelId string, masterPubKey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}

	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc),
		masterPubKey,
		nil,
		sp.Tsigner(pe.GetPID()),
		pubkey,
		privkey,
	)
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf("Error NewAuthSrv %v", err)
	}
	sc.SetAuthSrv(as)

	cksrv := newChunkSrv(kernelId, sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(path.Join(sp.CHUNKD, sc.ProcEnv().GetKernelID()), sc, cksrv)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	ssrv.RunServer()
}
