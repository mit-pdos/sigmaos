package fslib

import (
	"sigmaos/fcall"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

// Return pn, replacing first ~ip/~any with a symlink for a specific
// server. XXX remove duplication with pathclnt
func (fsl *FsLib) ResolveUnion(pn string) (string, bool, error) {
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		n, _, err := fsl.resolveUnion(d, left[0])
		if err != nil {
			return "", false, err
		}
		left[0] = n
		return d + "/" + left.String() + "/", ok, nil
	}
	return "", ok, nil
}

// Return pn but with all ~ip and ~any's replaced with symlinks for a
// specific server.
func (fsl *FsLib) ResolveUnions(pn string) (string, error) {
	for {
		npn, ok, err := fsl.ResolveUnion(pn)
		if err != nil {
			return "", err
		}
		if !ok {
			return pn, nil
		}
		pn = npn
	}
}

// Make copy of root mount or first union mount in pn. Return the
// content of symlink and the symlink's name.
func (fsl *FsLib) CopyMount(pn string) ([]byte, string, error) {
	if pn == sp.NAMED {
		return MakeTarget(Named()), "", nil
	}
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		_, b, err := fsl.resolveUnion(d, left[0])
		if err != nil {
			return nil, "", err
		}
		return b, left[1:].String(), nil
	}
	return nil, "", fcall.MkErr(fcall.TErrInval, pn)
}

// Return path to the symlink for the last server on this path and the
// the rest of the path on the server.
func (fsl *FsLib) PathLastSymlink(pn string) (string, path.Path, error) {
	// Make sure the server is automounted:
	if _, err := fsl.Stat(pn + "/"); err != nil {
		return "", nil, err
	}
	return fsl.LastMount(pn)
}

func (fsl *FsLib) resolveUnion(d string, q string) (string, []byte, error) {
	sts, err := fsl.GetDir(d)
	if err != nil {
		return "", nil, err
	}
	for _, st := range sts {
		b, err := fsl.GetFile(d + "/" + st.Name)
		if err != nil {
			continue
		}
		if ok := fsl.UnionMatch(q, string(b)); ok {
			return st.Name, b, nil
		}
	}
	return "", nil, fcall.MkErr(fcall.TErrNotfound, d)
}
