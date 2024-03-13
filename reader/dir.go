package reader

import (
	"bufio"
	"errors"
	"io"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

// Making rdr a bufio is important because the first read must be >=
// sizeof(st), because memfs and fsux try to avoid materializing
// directories as an array of bytes.
func MkDirReader(rdr io.Reader) *bufio.Reader {
	brdr := bufio.NewReader(rdr)
	return brdr
}

// Too stop early, f must return true.  Returns true if stopped early.
func ReadDir(drdr *bufio.Reader, f func(*sp.Stat) (bool, error)) (bool, error) {
	for {
		db.DPrintf(db.ALWAYS, "Unmarshal dir ent")
		st, err := spcodec.UnmarshalDirEnt(drdr)
		db.DPrintf(db.ALWAYS, "Unmarshal dir ent %v post err %v", st, err)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, err
		}
		stop, error := f(st)
		if stop {
			return true, error
		}
	}
	return false, nil
}
