package fslib

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) MkEndpointFile(pn string, ep *sp.Tendpoint, lid sp.TleaseId) error {
	if !ep.IsSigned() && fsl.ProcEnv().GetVerifyEndpoints() {
		db.DPrintf(db.ERROR, "Error make unsigned endpoint file")
		return fmt.Errorf("Unsigned endpoint: %v", ep)
	}
	b, err := ep.Marshal()
	if err != nil {
		return err
	}
	if err := fsl.PutFileAtomic(pn, 0777|sp.DMSYMLINK, b, lid); err != nil {
		db.DPrintf(db.ALWAYS, "PutFileAtomic %v err %v\n", pn, err)
		return err
	}
	return nil
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
	ep, error := sp.NewEndpointFromBytes(target)
	if error != nil {
		return &sp.Tendpoint{}, err
	}
	return ep, err
}

// Make copy of root endpoint or first endpoint in pn. Return the
// content of endpoint and the endpoint file's name.
func (fsl *FsLib) CopyEndpoint(pn string) (*sp.Tendpoint, string, error) {
	if pn == sp.NAMED {
		ep, err := fsl.FileAPI.GetNamedEndpoint()
		return ep, "", err
	}
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		_, ep, err := fsl.resolveMount(d, left[0])
		if err != nil {
			return nil, "", err
		}
		return ep, left[1:].String(), nil
	} else if s, p, err := fsl.FileAPI.PathLastMount(pn); err == nil {
		if ep, err := fsl.ReadEndpoint(s.String()); err == nil {
			return ep, p.String(), nil
		}
	}
	return nil, "", serr.NewErr(serr.TErrInval, pn)
}

func (fsl *FsLib) resolveMount(d string, q string) (string, *sp.Tendpoint, error) {
	var rep *sp.Tendpoint
	rname := ""
	// Make sure to resolve d in case it is a symlink or endpoint point.
	_, err := fsl.ProcessDir(d+"/", func(st *sp.Stat) (bool, error) {
		b, err := fsl.GetFile(d + "/" + st.Name)
		if err != nil {
			return false, nil
		}
		ep, error := sp.NewEndpointFromBytes(b)
		if error != nil {
			return false, nil
		}
		ok, err := fsl.FileAPI.IsLocalMount(ep)
		if err != nil {
			return false, err
		}
		if q == "~any" || ok {
			rname = st.Name
			rep = ep
			return true, nil
		}
		return false, nil
	})
	if err == nil && rname != "" {
		return rname, rep, nil
	}
	return rname, rep, serr.NewErr(serr.TErrNotfound, d)
}

// For code running using /ep/9p, which doesn't support PutFile.
func (fsl *FsLib) NewMount9P(pn string, ep *sp.Tendpoint) error {
	b, err := ep.Marshal()
	if err != nil {
		return err
	}
	return fsl.Symlink(b, pn, 0777|sp.DMTMP)
}
