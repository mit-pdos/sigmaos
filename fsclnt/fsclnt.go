package fsclnt

import (
	"errors"
	// "log"
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
	fds    []np.Tfid
	fids   map[np.Tfid]*Channel
	mounts map[string]np.Tfid
	cm     *ChanMgr
	Proc   string
	next   np.Tfid
}

type Channel struct {
	server string
	cname  []string
	qids   []np.Tqid
}

func makeChannel(s string, n []string, qs []np.Tqid) *Channel {
	c := &Channel{}
	c.server = s
	c.cname = n
	c.qids = qs
	return c
}

func MakeFsClient() *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]np.Tfid, 0, MAXFD)
	fsc.fids = make(map[np.Tfid]*Channel)
	fsc.cm = makeChanMgr()
	fsc.next = np.NoFid + 1
	fsc.mounts = make(map[string]np.Tfid)
	rand.Seed(time.Now().UnixNano())
	return fsc
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

func (fsc *FsClient) Close(fd int) error {
	if fsc.fds[fd] == np.NoFid {
		return errors.New("Close: fd isn't open")
	}
	fid := fsc.fds[fd]
	args := np.Tclunk{np.NoTag, fid}
	var reply np.Rclunk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Clunk", args, &reply)
	fsc.fds[fd] = np.NoFid
	return err
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

func (fsc *FsClient) Attach(server string, path string) (int, error) {
	fid := fsc.allocFid()
	p := strings.Split(path, "/")
	args := np.Tattach{np.NoTag, fid, np.NoFid, "fk", ""}
	var reply np.Rattach
	err := fsc.cm.makeCall(server, "FsConn.Attach", args, &reply)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid] = makeChannel(server, p, []np.Tqid{reply.Qid})
	fd := fsc.findfd(fid)
	return fd, nil
}

func (fsc *FsClient) Mount(fd int, path string) error {
	fsc.mounts[path] = fsc.fds[fd]
	return nil
}

func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, error) {
	c := fsc.fids[fid]
	fid1 := fsc.allocFid()
	var qids []np.Tqid
	copy(c.qids, qids)
	fsc.fids[fid1] = makeChannel(c.server, c.cname, qids)
	return fid1, nil
}

func (fsc *FsClient) mount2fid(path []string) (np.Tfid, []string, error) {
	return np.NoFid, nil, nil
}

func (fsc *FsClient) walk(fid np.Tfid, nfid np.Tfid, path []string) (*np.Rwalk, error) {
	args := np.Twalk{np.NoTag, fid, nfid, nil}
	var reply np.Rwalk
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Twalk", args, &reply)
	return &reply, err
}

func (fsc *FsClient) create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (*np.Rcreate, error) {
	args := np.Tcreate{np.NoTag, fid, name, perm, mode}
	var reply np.Rcreate
	err := fsc.cm.makeCall(fsc.fids[fid].server, "FsConn.Tcreate", args, &reply)
	return &reply, err
}

func (fsc *FsClient) Create(path string) (int, error) {
	p := strings.Split(path, "/")
	fid, rest, err := fsc.mount2fid(p)
	fid1 := fsc.allocFid()
	_, err = fsc.walk(fid, fid1, nil)
	if err != nil {
		return -1, err
	}
	fid2 := fsc.allocFid()
	_, err = fsc.walk(fid1, fid2, rest)
	if err != nil {
		return -1, err
	}
	return 0, nil
}
