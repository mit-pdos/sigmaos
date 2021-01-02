package fsclnt

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	//"strconv"
	"strings"
	"time"

	np "ulambda/ninep"
)

const (
	Stdin  = 0
	Stdout = 1
	// Stderr = 2
)

const MAXFD = 20

type FsClient struct {
	fds   []np.Tfid
	fids  map[np.Tfid]*Channel
	mount *Mount
	cm    *ChanMgr
	Proc  string
	next  np.Tfid
}

func MakeFsClient() *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]np.Tfid, 0, MAXFD)
	fsc.fids = make(map[np.Tfid]*Channel)
	fsc.mount = makeMount()
	fsc.cm = makeChanMgr()
	fsc.next = np.NoFid + 1
	rand.Seed(time.Now().UnixNano())
	return fsc
}

func (fsc *FsClient) String() string {
	str := fmt.Sprintf("Fsclnt table:\n")
	str += fmt.Sprintf("fds %v\n", fsc.fds)
	for k, v := range fsc.fids {
		str += fmt.Sprintf("fid %v chan %v", k, v)
	}
	return str
}

// // XXX use gob?
// func InitFsClient(root *fid.Ufid, args []string) (*FsClient, []string, error) {
// 	log.Printf("InitFsClient: %v\n", args)
// 	if len(args) < 2 {
// 		return nil, nil, errors.New("Missing len and program")
// 	}
// 	n, err := strconv.Atoi(args[0])
// 	if err != nil {
// 		return nil, nil, errors.New("Bad arg len")
// 	}
// 	if n < 1 {
// 		return nil, nil, errors.New("Missing program")
// 	}
// 	a := args[1 : n+1] // skip len and +1 for program name
// 	fids := args[n+1:]
// 	fsc := MakeFsClient(root)
// 	fsc.Proc = a[0]
// 	log.Printf("Args %v fids %v\n", a, fids)
// 	for _, f := range fids {
// 		var uf fid.Ufid
// 		err := json.Unmarshal([]byte(f), &uf)
// 		if err != nil {
// 			return nil, nil, errors.New("Bad fid")
// 		}
// 		fsc.findfd(&uf)
// 	}
// 	return fsc, a, nil
// }

func (fsc *FsClient) findfd(nfid np.Tfid) int {
	for fd, fid := range fsc.fds {
		if fid == np.NoFid {
			fsc.fds[fd] = nfid
			return fd
		}
	}
	// no free one
	fsc.fds = append(fsc.fds, nfid)
	return len(fsc.fds) - 1
}

func (fsc *FsClient) allocFid() np.Tfid {
	fid := fsc.next
	fsc.next += 1
	return fid
}

func (fsc *FsClient) lookup(fd int) (np.Tfid, error) {
	if fsc.fds[fd] == np.NoFid {
		return np.NoFid, errors.New("Non-existing")
	}
	return fsc.fds[fd], nil
}

func (fsc *FsClient) Mount(fd int, path string) error {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return err
	}
	fsc.mount.add(strings.Split(path, "/"), fid)
	return nil
}

func (fsc *FsClient) Close(fd int) error {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return err
	}
	err = fsc.clunk(fid)
	if err == nil {
		fsc.fds[fd] = np.NoFid
	}
	return err
}

func (fsc *FsClient) Attach(server string, path string) (int, error) {
	fid := fsc.allocFid()
	p := strings.Split(path, "/")
	reply, err := fsc.attach(server, fid, p)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid] = makeChannel(server, p, []np.Tqid{reply.Qid})
	fd := fsc.findfd(fid)
	log.Printf("Attach -> fd %v fid %v %v\n", fd, fid, fsc.fids[fid])
	return fd, nil
}

func (fsc *FsClient) nameFid(path []string) (np.Tfid, error) {
	fid, rest := fsc.mount.resolve(path)
	if fid == np.NoFid {
		return np.NoFid, errors.New("Unknown file")

	}

	// clone fid into fid1
	fid1 := fsc.allocFid()
	_, err := fsc.walk(fid, fid1, nil)
	if err != nil {
		return np.NoFid, err
	}
	fsc.fids[fid1] = fsc.fids[fid].copyChannel()

	defer func() {
		err := fsc.clunk(fid1)
		if err != nil {
			log.Printf("Create clunk failed %v\n", err)
		}
		delete(fsc.fids, fid1)
	}()

	fid2 := fsc.allocFid()
	reply, err := fsc.walk(fid1, fid2, rest)
	if err != nil {
		return np.NoFid, err
	}
	fsc.fids[fid2] = fsc.fids[fid1].copyChannel()
	fsc.fids[fid2].addn(rest, reply.Qids)
	return fid2, nil
}

func (fsc *FsClient) Open(path string, mode np.Tmode) (int, error) {
	fid, err := fsc.nameFid(strings.Split(path, "/"))
	if err != nil {
		return -1, err
	}
	_, err = fsc.open(fid, mode)
	if err != nil {
		return -1, err
	}
	// XXX check reply.Qid?
	fd := fsc.findfd(fid)
	return fd, nil

}

func (fsc *FsClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	log.Printf("Create %v\n", path)

	p := strings.Split(path, "/")
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.nameFid(dir)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.create(fid, base, perm, mode)
	if err != nil {
		return -1, err
	}

	fsc.fids[fid].add(base, reply.Qid)
	fd := fsc.findfd(fid)

	log.Printf("fsc %v\n", fsc)

	return fd, nil
}

func (fsc *FsClient) Lsof() []string {
	var fids []string
	for _, fid := range fsc.fds {
		if fid != np.NoFid {
			// collect info about fid...
			//b, err := json.Marshal(fid)
			//if err != nil {
			//	log.Fatal("Marshall error:", err)
			//}
			//fids = append(fids, string(b))
		}
	}
	return fids
}
