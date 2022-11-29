package hotel

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
)

const (
	HOTEL      = "hotel"
	HOTELDIR   = "name/hotel/"
	MEMFS      = "memfs"
	HTTP_ADDRS = "http-addr"
	NCACHE     = 3
)

func JobDir(job string) string {
	return path.Join(HOTELDIR, job)
}

func JobHTTPAddrsPath(job string) string {
	return path.Join(JobDir(job), HTTP_ADDRS)
}

func MemFsPath(job string) string {
	return path.Join(JobDir(job), MEMFS)
}

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) ([]string, error) {
	p := JobHTTPAddrsPath(job)
	var addrs []string
	err := fsl.GetFileJson(p, &addrs)
	return addrs, err
}

func InitHotelFs(fsl *fslib.FsLib, jobname string) {
	fsl.MkDir(HOTELDIR, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		db.DFatalf("Mkdir %v err %v\n", JobDir(jobname), err)
	}
}
