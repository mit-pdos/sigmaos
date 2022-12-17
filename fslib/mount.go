package fslib

import (
	"sigmaos/fcall"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

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

// Return pn, replacing first ~ip, if any, with a symlink for local
// server XXX remove duplication with pathclnt
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

// Make copy of root or first union mount in pn
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
