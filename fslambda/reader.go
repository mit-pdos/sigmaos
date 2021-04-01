package fslambda

import (
	"errors"
	"log"
	// "time"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

type Reader struct {
	*fslib.FsLibSrv
	pid    string
	input  string
	output string
	pipe   npo.NpObj
}

func MakeReader(args []string) (*Reader, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeReader: %v\n", args)

	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, errors.New("MakeReader: No IP")
	}
	n := "name/" + args[2]
	memfsd := memfsd.MakeFsd(ip+":0", nil)
	pipe, err := memfsd.MkPipe("pipe")
	if err != nil {
		log.Fatal("Create error: ", err)
	}

	fsl, err := fslib.InitFs(n, memfsd, nil)
	if err != nil {
		return nil, err
	}

	r := &Reader{}
	r.FsLibSrv = fsl
	r.pid = args[0]
	r.input = args[1]
	r.output = args[2]
	r.pipe = pipe
	r.Started(r.pid)

	return r, nil
}

func (r *Reader) Work() {
	db.DLPrintf("Reader", "Reader: work\n")
	err := r.pipe.Open(nil, np.OWRITE)
	if err != nil {
		log.Fatal("Open error: ", err)
	}
	fd, err := r.Open(r.input, np.OREAD)
	if err != nil {
		log.Fatal(err)
	}
	for {
		data, err := r.Read(fd, memfs.PIPESZ)
		if len(data) == 0 {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		_, err = r.pipe.(npo.NpObjFile).Write(nil, 0, data, np.NoV)
		if err != nil {
			log.Fatal(err)
		}
	}
	r.Close(fd)
	r.pipe.Close(nil, np.OWRITE)

	r.ExitFs("name/" + r.output)
	r.Exiting(r.pid, "OK")
}
