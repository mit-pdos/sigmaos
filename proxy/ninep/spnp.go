package npproxysrv

import (
	"bytes"

	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/fslib/reader"
	np "sigmaos/ninep"
	"sigmaos/proxy/ninep/npcodec"
	sp "sigmaos/sigmap"
)

func Sp2NpDir(d []byte, cnt sp.Tsize) ([]byte, error) {
	rdr := bytes.NewReader(d)
	npsts := make([]*np.Stat9P, 0)
	_, error := reader.ReadDirEnts(reader.MkDirEntsReader(rdr), func(st *sp.Tstat) (bool, error) {
		npst := npcodec.Sp2NpStat(st.StatProto())
		npsts = append(npsts, npst)
		return false, nil
	})
	if error != nil {
		db.DPrintf(db.NPPROXY, "Sp2NpDir: %d errror %v\n", len(npsts), error)
		return nil, error
	}
	d1, _, err := fs.MarshalDir(cnt, npsts)
	return d1, err
}
