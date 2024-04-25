package fslib

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) MkEndpointFile(pn string, mnt *sp.Tendpoint, lid sp.TleaseId) error {
	if !mnt.IsSigned() && fsl.ProcEnv().GetVerifyEndpoints() {
		db.DPrintf(db.ERROR, "Error make unsigned endpoint file")
		return fmt.Errorf("Unsigned endpoint: %v", mnt)
	}
	b, err := mnt.Marshal()
	if err != nil {
		return err
	}
	if err := fsl.PutFileAtomic(pn, 0777|sp.DMSYMLINK, b, lid); err != nil {
		db.DPrintf(db.ALWAYS, "PutFileAtomic %v err %v\n", pn, err)
		return err
	}
	return nil
}

func (fsl *FsLib) RemoveMount(pn string) error {
	db.DPrintf(db.ALWAYS, "RemoveMount %v\n", pn)
	return fsl.Remove(pn)
}

// Return pn, replacing first ~local/~any with a endpoint point for a specific
// server.
func (fsl *FsLib) ResolveMount(pn string) (string, bool, error) {
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		n, _, err := fsl.resolveMount(d, left[0])
		if err != nil {
			return "", false, err
		}
		left[0] = n
		return d + "/" + left.String() + "/", ok, nil
	}
	return "", ok, nil
}

// Return pn but with all ~local and ~any's replaced with endpoint points for a
// specific server.
func (fsl *FsLib) ResolveMounts(pn string) (string, error) {
	for {
		npn, ok, err := fsl.ResolveMount(pn)
		if err != nil {
			return "", err
		}
		if !ok {
			return pn, nil
		}
		pn = npn
	}
}

func (fsl *FsLib) ReadEndpoint(pn string) (*sp.Tendpoint, error) {
	target, err := fsl.GetFile(pn)
	if err != nil {
		return &sp.Tendpoint{}, err
	}
	mnt, error := sp.NewEndpointFromBytes(target)
	if error != nil {
		return &sp.Tendpoint{}, err
	}
	return mnt, err
}

// Make copy of root endpoint or first endpoint in pn. Return the
// content of endpoint and the endpoint file's name.
func (fsl *FsLib) CopyEndpoint(pn string) (*sp.Tendpoint, string, error) {
	if pn == sp.NAMED {
		mnt, err := fsl.SigmaOS.GetNamedEndpoint()
		return mnt, "", err
	}
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		_, mnt, err := fsl.resolveMount(d, left[0])
		if err != nil {
			return sp.NewNullEndpoint(), "", err
		}
		return mnt, left[1:].String(), nil
	} else if s, p, err := fsl.SigmaOS.PathLastMount(pn); err == nil {
		if mnt, err := fsl.ReadEndpoint(s.String()); err == nil {
			return mnt, p.String(), nil
		}
	}
	return sp.NewNullEndpoint(), "", serr.NewErr(serr.TErrInval, pn)
}

func (fsl *FsLib) resolveMount(d string, q string) (string, *sp.Tendpoint, error) {
	rmnt := sp.NewNullEndpoint()
	rname := ""
	// Make sure to resolve d in case it is a symlink or endpoint point.
	_, err := fsl.ProcessDir(d+"/", func(st *sp.Stat) (bool, error) {
		b, err := fsl.GetFile(d + "/" + st.Name)
		if err != nil {
			return false, nil
		}
		mnt, error := sp.NewEndpointFromBytes(b)
		if error != nil {
			return false, nil
		}
		ok, err := fsl.SigmaOS.IsLocalMount(mnt)
		if err != nil {
			return false, err
		}
		if q == "~any" || ok {
			rname = st.Name
			rmnt = mnt
			return true, nil
		}
		return false, nil
	})
	if err == nil && rname != "" {
		return rname, rmnt, nil
	}
	return rname, rmnt, serr.NewErr(serr.TErrNotfound, d)
}

// For code running using /mnt/9p, which doesn't support PutFile.
func (fsl *FsLib) NewMount9P(pn string, mnt *sp.Tendpoint) error {
	b, err := mnt.Marshal()
	if err != nil {
		return err
	}
	return fsl.Symlink(b, pn, 0777|sp.DMTMP)
}
