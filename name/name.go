package name

import (
	"errors"
	"log"
	"path/filepath"
	"strings"

	"ulambda/fid"
)

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type Root struct {
	root    *fid.Ufid
	dir     *Dir
	inodes  map[fid.Fid]*Inode
	rFid    fid.Fid
	nextFid fid.Fid
}

func MakeRoot(root *fid.Ufid) *Root {
	r := Root{}
	r.dir = makeDir()
	r.root = root
	r.inodes = make(map[fid.Fid]*Inode)
	r.rFid = fid.RootFid()
	r.nextFid = fid.MakeFid(0, r.rFid.Id+1)
	r.inodes[r.rFid] = makeInode(fid.DirT, r.rFid, r.dir)
	return &r
}

func (root *Root) Myroot() *fid.Ufid {
	return root.root
}

// XXX bump version # if allocating same inode #
// XXX a better inum allocation plan
func (root *Root) allocInum() fid.Fid {
	fid := root.nextFid
	root.nextFid.Id += 1
	return fid
}

func (root *Root) makeUfid(fid fid.Fid) *fid.Ufid {
	ufid := *root.root
	ufid.Fid = fid
	return &ufid
}

func (root *Root) RootFid() fid.Fid {
	return root.rFid
}

func (root *Root) fid2Inode(f fid.Fid) (*Inode, error) {
	if inode, ok := root.inodes[f]; ok {
		return inode, nil
	} else {
		return nil, errors.New("Unknown fid")
	}
}

func (root *Root) fid2Dir(f fid.Fid) (*Dir, error) {
	inode, err := root.fid2Inode(f)
	if err != nil {
		return nil, err
	}
	if inode.Type == fid.DirT {
		return inode.Data.(*Dir), nil
	} else {
		return nil, errors.New("Not a directory")
	}
}

func (root *Root) Namei(start fid.Fid, path []string) (*Inode, []string, error) {
	if IsCurrentDir(path[0]) {
		i, err := root.fid2Inode(start)
		return i, path[1:], err
	}
	dir, err := root.fid2Dir(start)
	if err != nil {
		return nil, nil, err
	}
	return dir.Namei(path)
}

func (root *Root) Walk(start fid.Fid, path string) (*fid.Ufid, string, error) {
	log.Printf("name.Walk %v\n", path)
	inode, rest, err := root.Namei(start, strings.Split(path, "/"))
	if err == nil {
		switch inode.Type {
		case fid.MountT:
			uf := inode.Data.(*fid.Ufid)
			return uf, strings.Join(rest, "/"), err
		case fid.SymT:
			s := inode.Data.(*Symlink)
			return s.start, s.dst + "/" + strings.Join(rest, "/"), err
		default:
			return root.makeUfid(inode.Fid), strings.Join(rest, "/"), err
		}
	} else {
		return nil, "", err
	}
}

func (root *Root) OpenFid(f fid.Fid) (*Inode, error) {
	return root.fid2Inode(f)
}

func (root *Root) WalkOpenFid(start fid.Fid, path string) (*Inode, error) {
	ufid, rest, err := root.Walk(start, path)
	if err != nil {
		return nil, err
	}
	if rest != "" {
		return nil, errors.New("Unknow file")

	}
	return root.fid2Inode(ufid.Fid)
}

func (root *Root) Ls(f fid.Fid, n int) (string, error) {
	dir, err := root.fid2Dir(f)
	if err != nil {
		return "", err
	}
	return dir.ls(n), nil
}

func (root *Root) create(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, fid.FileT, base, i)
}

func (root *Root) mkdir(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, fid.DirT, base, i)
}

func (root *Root) symlink(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, fid.SymT, base, i)
}

func (root *Root) mount(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, fid.MountT, base, i)
}

func (root *Root) mknod(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, fid.DevT, base, i)
}

func (root *Root) pipe(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, fid.PipeT, base, i)
}

func (root *Root) Op(opn string, start fid.Fid, path string,
	op func(*Inode, string, interface{}) (*Inode, error),
	i interface{}) (*Inode, error) {
	log.Printf("name.%v %v %v %v\n", opn, start, path, i)
	if inode, _, err := root.Namei(start,
		strings.Split(filepath.Dir(path), "/")); err == nil {
		inode, err = op(inode, filepath.Base(path), i)
		if err != nil {
			return inode, err
		}
		log.Printf("name.%v %v %v %v -> %v\n", opn, start, path, i, inode)
		root.inodes[inode.Fid] = inode
		return inode, nil
	} else {
		return nil, err
	}
}

func (root *Root) Create(start fid.Fid, path string) (*Inode, error) {
	return root.Op("Create", start, path, root.create, []byte{})
}

func (root *Root) MkDir(start fid.Fid, path string) (*Inode, error) {
	return root.Op("MkDir", start, path, root.mkdir, makeDir())
}

func (root *Root) Symlink(start fid.Fid, src string, dstart *fid.Ufid, dst string) (*Inode, error) {
	return root.Op("Symlink", start, src, root.symlink, makeSym(dstart, dst))
}

func (root *Root) Mount(uf *fid.Ufid, start fid.Fid, path string) error {
	_, err := root.Op("Mount", start, path, root.mount, uf)
	return err
}

func (root *Root) MkNod(start fid.Fid, path string, rw Dev) error {
	_, err := root.Op("Mknod", start, path, root.mknod, rw)
	return err
}

func (root *Root) Pipe(start fid.Fid, path string) error {
	_, err := root.Op("Pipe", start, path, root.pipe, makePipe())
	return err
}

func (root *Root) Write(f fid.Fid, data []byte) (int, error) {
	log.Printf("name.Write %v\n", f)
	inode, err := root.fid2Inode(f)
	if err != nil {
		return 0, err
	}
	// XXX no distinction between DevT and pipeT?
	if inode.Type == fid.DevT {
		dev := inode.Data.(Dev)
		return dev.Write(f, data)
	} else if inode.Type == fid.PipeT {
		pipe := inode.Data.(*Pipe)
		return pipe.Write(f, data)
	} else {
		inode.Data = data
		return len(data), nil
	}
}

func (root *Root) Read(f fid.Fid, n int) ([]byte, error) {
	log.Printf("name.Read %v\n", f)
	inode, err := root.fid2Inode(f)
	if err != nil {
		return nil, err
	}
	switch inode.Type {
	case fid.DevT:
		dev := inode.Data.(Dev)
		return dev.Read(f, n)
	case fid.PipeT:
		pipe := inode.Data.(*Pipe)
		return pipe.Read(f, n)
	case fid.FileT:
		return inode.Data.([]byte), nil
	case fid.DirT:
		dir := inode.Data.(*Dir)
		return []byte(dir.ls(n)), nil
	default:
		return nil, errors.New("Unreadable fid")
	}
}
