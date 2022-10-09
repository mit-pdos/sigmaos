package www

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
)

const (
	WWWD       = "wwwd"
	WWWDIR     = "name/www/"
	MEMFS      = "memfs"
	HTTP_ADDRS = "http-addr"
)

func JobDir(job string) string {
	return path.Join(WWWDIR, job)
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
	err := fsl.GetFileJson(p, addrs)
	return addrs, err
}

func InitWwwFs(fsl *fslib.FsLib, jobname string) {
	fsl.MkDir(WWWDIR, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		db.DFatalf("Mkdir %v err %v\n", JobDir(jobname), err)
	}
}
