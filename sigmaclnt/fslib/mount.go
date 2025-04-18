package fslib

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) MkLeasedEndpoint(pn sp.Tsigmapath, ep *sp.Tendpoint, lid sp.TleaseId) error {
	b, err := ep.Marshal()
	if err != nil {
		return err
	}
	if _, err := fsl.PutLeasedFile(pn, 0777|sp.DMSYMLINK, sp.OWRITE|sp.OEXCL, lid, b); err != nil {
		db.DPrintf(db.ALWAYS, "MkLeasedEndpoint: %v err %v\n", pn, err)
		return err
	}
	return nil
}

func (fsl *FsLib) MkEndpointFile(pn sp.Tsigmapath, ep *sp.Tendpoint) error {
	b, err := ep.Marshal()
	if err != nil {
		return err
	}
	if _, err := fsl.PutFile(pn, 0777|sp.DMSYMLINK, sp.OWRITE|sp.OEXCL, b); err != nil {
		db.DPrintf(db.ALWAYS, "MkEndpointFile: %v err %v\n", pn, err)
		return err
	}
	return nil
}

func (fsl *FsLib) WriteEndpointFile(pn sp.Tsigmapath, ep *sp.Tendpoint) error {
	b, err := ep.Marshal()
	if err != nil {
		return err
	}
	if err := fsl.PutFileAtomic(pn, 0777|sp.DMSYMLINK, b); err != nil {
		db.DPrintf(db.ALWAYS, "WriteEndpointFile %v err %v\n", pn, err)
		return err
	}
	return nil
}

// Return pn, replacing first LOCAL/ANY with a endpoint point for a specific
// server.
func (fsl *FsLib) ResolveMount(pn sp.Tsigmapath) (sp.Tsigmapath, bool, error) {
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

// Return pn but with all LOCAL and ANY's replaced with endpoint points for a
// specific server.
func (fsl *FsLib) ResolveMounts(pn sp.Tsigmapath) (sp.Tsigmapath, error) {
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

func (fsl *FsLib) ReadEndpoint(pn sp.Tsigmapath) (*sp.Tendpoint, error) {
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
func (fsl *FsLib) CopyEndpoint(pn sp.Tsigmapath) (*sp.Tendpoint, sp.Tsigmapath, error) {
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

func (fsl *FsLib) isLocal(pn sp.Tsigmapath) (bool, *sp.Tendpoint, error) {
	b, err := fsl.GetFile(pn)
	if err != nil {
		return false, nil, err
	}
	ep, error := sp.NewEndpointFromBytes(b)
	if error != nil {
		return false, nil, err
	}
	ok, err := fsl.FileAPI.IsLocalMount(ep)
	if err != nil {
		return false, nil, err
	}
	return ok, ep, nil
}

func (fsl *FsLib) resolveMount0(d sp.Tsigmapath, q sp.Tsigmapath) (sp.Tsigmapath, *sp.Tendpoint, error) {
	var rep *sp.Tendpoint
	rname := ""
	// Make sure to resolve d in case it is a symlink or endpoint point.
	_, err := fsl.ProcessDir(d+"/", func(st *sp.Tstat) (bool, error) {
		ok, ep, err := fsl.isLocal(filepath.Join(d, st.Name))
		if err != nil {
			return false, err
		}
		if q == sp.ANY || ok {
			db.DPrintf(db.TEST, "resolveMount %v %v %v", d, st.Name, fsl.pe.GetKernelID())
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

func (fsl *FsLib) resolveMount(d sp.Tsigmapath, q sp.Tsigmapath) (sp.Tsigmapath, *sp.Tendpoint, error) {
	rname := ""

	if fsl.pe.GetKernelID() != sp.NOT_SET {
		rname = fsl.pe.GetKernelID()
		ok, ep, err := fsl.isLocal(filepath.Join(d, rname))
		if err != nil {
			return "", nil, err
		}
		if !ok {
			db.DFatalf("resolveMount %v isn't local %v", rname, ep)
		}
		if q == sp.ANY || ok {
			return rname, ep, nil
		}
	}
	return fsl.resolveMount0(d, q)
}

// For code running using /mnt/9p, which doesn't support PutFile.
func (fsl *FsLib) NewMount9P(pn sp.Tsigmapath, ep *sp.Tendpoint) error {
	b, err := ep.Marshal()
	if err != nil {
		return err
	}
	return fsl.Symlink(b, pn, 0777)
}
