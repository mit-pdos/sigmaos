package hotel

import (
	"path"

	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
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

func MakeHotelJob(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, job string, srvs []string, ncore proc.Tcore, ncache int) (*cacheclnt.CacheClnt, *cacheclnt.CacheMgr, []proc.Tpid, error) {
	var cc *cacheclnt.CacheClnt
	var cm *cacheclnt.CacheMgr
	var err error

	// Init fs.
	InitHotelFs(fsl, job)

	// Create a cache clnt.
	if ncache > 0 {
		cm = cacheclnt.MkCacheMgr(fsl, pclnt, job, ncache)
		cm.StartCache()
		cc, err = cacheclnt.MkCacheClnt(fsl, ncache)
		if err != nil {
			db.DFatalf("Error cacheclnt %v", err)
			return nil, nil, nil, err
		}
	}

	pids := make([]proc.Tpid, 0, len(srvs))

	for _, srv := range srvs {
		p := proc.MakeProc(srv, []string{job})
		p.SetNcore(ncore)
		if err = pclnt.Spawn(p); err != nil {
			db.DFatalf("Error spawn proc %v: %v", p, err)
			return nil, nil, nil, err
		}
		if err = pclnt.WaitStart(p.Pid); err != nil {
			db.DFatalf("Error spawn proc %v: %v", p, err)
			return nil, nil, nil, err
		}
		pids = append(pids, p.Pid)
	}

	return cc, cm, pids, nil
}
