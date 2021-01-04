package fsclnt

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	np "ulambda/ninep"
)

const (
	// zero channel to named
	Stdin  = 1
	Stdout = 2
	// Stderr = 2
)

const MAXFD = 20

const MAXSYMLINK = 4

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
		str += fmt.Sprintf("fid %v chan %v\n", k, v)
	}
	return str
}

// // XXX use gob?
func InitFsClient(args []string) (*FsClient, error) {
	log.Printf("InitFsClient: %v\n", args)
	if len(args) < 2 {
		return nil, errors.New("Missing len and program")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, errors.New("Bad arg len")
	}
	if n < 1 {
		return nil, errors.New("Missing program")
	}
	a := args[1 : n+1] // skip len and +1 for program name
	fids := args[n+1:]
	fsc := MakeFsClient()
	fsc.Proc = a[0]
	log.Printf("Args %v fids %v\n", a, fids)
	if fd, err := fsc.Attach(":1111", ""); err == nil {
		err := fsc.Mount(fd, "name")
		if err != nil {
			return nil, errors.New("Mount error")
		}
		_, err = fsc.Open("name/consoled/console", np.OREAD)
		if err != nil {
			return nil, errors.New("Open error")
		}
		_, err = fsc.Open("name/consoled/console", np.OWRITE)
		if err != nil {
			return nil, errors.New("Open error")
		}
	}

	// for _, f := range fids {
	// 	var uf fid.Ufid
	// 	err := json.Unmarshal([]byte(f), &uf)
	// 	if err != nil {
	// 		return nil, nil, errors.New("Bad fid")
	// 	}
	// 	fsc.findfd(&uf)
	// }

	return fsc, nil
}

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

func split(path string) []string {
	p := strings.Split(path, "/")
	return p
}

func join(path []string) string {
	p := strings.Join(path, "/")
	return p
}

func (fsc *FsClient) Mount(fd int, path string) error {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return err
	}
	log.Printf("Mount %v at %v\n", fid, path)
	fsc.mount.add(split(path), fid)
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

func (fsc *FsClient) AttachChannel(fid np.Tfid, server string, p []string) (*Channel, error) {
	reply, err := fsc.attach(server, fid, p)
	if err != nil {
		return nil, err
	}
	return makeChannel(server, p, []np.Tqid{reply.Qid}), nil
}

func (fsc *FsClient) Attach(server string, path string) (int, error) {
	log.Printf("Attach %v %v\n", server, path)
	p := split(path)
	fid := fsc.allocFid()
	ch, err := fsc.AttachChannel(fid, server, p)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid] = ch
	fd := fsc.findfd(fid)
	log.Printf("Attach -> fd %v fid %v %v\n", fd, fid, fsc.fids[fid])
	return fd, nil
}

// clone fid into fid1
func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, error) {
	fid1 := fsc.allocFid()
	log.Printf("clone fid %v -> fid1 %v\n", fid, fid1)
	_, err := fsc.walk(fid, fid1, nil)
	if err != nil {
		// XXX free fid
		return np.NoFid, err
	}
	fsc.fids[fid1] = fsc.fids[fid].copyChannel()
	return fid1, err
}

func (fsc *FsClient) closeFid(fid np.Tfid) {
	err := fsc.clunk(fid)
	if err != nil {
		log.Printf("closeFid clunk failed %v\n", err)
	}
	delete(fsc.fids, fid)
}

func (fsc *FsClient) walkOne(path []string) (np.Tfid, int, error) {
	fid, rest := fsc.mount.resolve(path)
	if fid == np.NoFid {
		return np.NoFid, 0, errors.New("Unknown file")

	}
	fid1, err := fsc.clone(fid)
	if err != nil {
		return np.NoFid, 0, err
	}
	defer fsc.closeFid(fid1)

	fid2 := fsc.allocFid()
	log.Printf("walk fid1 %v fid2 %v\n", fid1, fid2)
	reply, err := fsc.walk(fid1, fid2, rest)
	if err != nil {
		return np.NoFid, 0, err
	}
	todo := len(rest) - len(reply.Qids)
	log.Printf("walkOne rest %v -> %v %v", rest, reply.Qids, todo)

	fsc.fids[fid2] = fsc.fids[fid1].copyChannel()
	fsc.fids[fid2].addn(reply.Qids, rest)
	return fid2, todo, nil
}

// XXX more robust impl
func splitTarget(target string) (string, string) {
	parts := strings.Split(target, ":")
	server := parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + parts[3]
	return server, parts[len(parts)-1]
}

func (fsc *FsClient) walkMany(path []string) (np.Tfid, error) {
	start := path
	for i := 0; i < MAXSYMLINK; i++ {
		fid, todo, err := fsc.walkOne(start)
		if err != nil {
			return fid, err
		}
		qid := fsc.fids[fid].lastqid()
		if qid.Type == np.QTSYMLINK {
			reply, err := fsc.readlink(fid)
			if err != nil {
				return np.NoFid, err
			}
			server, _ := splitTarget(reply.Target)
			fd, err := fsc.Attach(server, "")
			if err != nil {
				log.Fatal("Attach error: ", err)
			}
			i := len(path) - todo
			err = fsc.Mount(fd, join(path[:i]))
			if err != nil {
				log.Fatal("Mount error: ", err)
			}
			start = path
		} else {
			return fid, err

		}
	}
	return np.NoFid, errors.New("too many iterations")
}

func (fsc *FsClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	log.Printf("Create %v\n", path)
	p := split(path)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.create(fid, base, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid].add(base, reply.Qid)
	fd := fsc.findfd(fid)

	return fd, nil
}

func (fsc *FsClient) CreateAt(dfd int, name string, perm np.Tperm, mode np.Tmode) (int, error) {
	log.Printf("CreateAt %v at %v\n", name, dfd)
	fid, err := fsc.lookup(dfd)
	if err != nil {
		return -1, err
	}

	fid1, err := fsc.clone(fid)
	if err != nil {
		return -1, err
	}

	reply, err := fsc.create(fid1, name, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid1].add(name, reply.Qid)
	fd := fsc.findfd(fid1)
	return fd, nil
}

func (fsc *FsClient) Symlink(target string, link string) error {
	log.Printf("Symlink %v %v\n", target, link)
	p := split(link)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir)
	if err != nil {
		return err
	}
	// XXX maybe do something with returned qid
	_, err = fsc.symlink(fid, base, target)
	return err
}

func (fsc *FsClient) SymlinkAt(dfd int, target string, linkn string) error {
	log.Printf("SymlinkAt %v %v\n", target, linkn)
	fid, err := fsc.lookup(dfd)
	if err != nil {
		return err
	}
	// XXX maybe do something with returned qid
	_, err = fsc.symlink(fid, linkn, target)
	if err != nil {
		return nil
	}
	return err
}

func (fsc *FsClient) Mkdir(path string, mode np.Tmode) (int, error) {
	log.Printf("Mkdir %v\n", path)

	log.Printf("fsc %v\n", fsc)
	p := split(path)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.mkdir(fid, base, mode)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid].add(base, reply.Qid)
	fd := fsc.findfd(fid)
	log.Printf("fsc mkdir done %v\n", fsc)
	return fd, err
}

func (fsc *FsClient) Pipe(path string, mode np.Tmode) error {
	log.Printf("Mkpipe %v\n", path)
	p := split(path)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir)
	if err != nil {
		return err
	}
	_, err = fsc.mkpipe(fid, base, mode)
	return err
}

func (fsc *FsClient) Open(path string, mode np.Tmode) (int, error) {
	log.Printf("Open %v %v\n", path, mode)
	fid, err := fsc.walkMany(split(path))
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

func (fsc *FsClient) OpenAt(dfd int, name string, mode np.Tmode) (int, error) {
	log.Printf("OpenAt %v %v %v\n", dfd, name, mode)

	fid, err := fsc.lookup(dfd)
	if err != nil {
		return -1, err
	}

	fid1, err := fsc.clone(fid)
	if err != nil {
		return -1, err
	}

	n := []string{name}
	reply, err := fsc.walk(fid, fid1, n)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid1].addn(reply.Qids, n)

	_, err = fsc.open(fid1, mode)
	if err != nil {
		return -1, err
	}
	// XXX check reply.Qid?
	fd := fsc.findfd(fid1)
	return fd, nil

}

func (fsc *FsClient) Read(fd int, offset np.Toffset, cnt np.Tsize) ([]byte, error) {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return nil, err
	}
	reply, err := fsc.read(fid, offset, cnt)
	return reply.Data, err
}

func (fsc *FsClient) Write(fd int, offset np.Toffset, data []byte) (np.Tsize, error) {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return 0, err
	}
	reply, err := fsc.write(fid, offset, data)
	return reply.Count, err
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
