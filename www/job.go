package www

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
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

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) (sp.Taddrs, error) {
	mnt, err := fsl.ReadMount(JobHTTPAddrsPath(job))
	if err != nil {
		return nil, err
	}
	return mnt.Addresses(), err
}

func InitWwwFs(fsl *fslib.FsLib, jobname string) error {
	fsl.MkDir(WWWDIR, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		db.DPrintf(db.ERROR, "Mkdir %v err %v\n", JobDir(jobname), err)
		return err
	}
	return nil
}
