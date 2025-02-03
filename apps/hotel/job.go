package hotel

import (
	"os"
	"path/filepath"
	"strconv"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	"sigmaos/apps/kv"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

const (
	HOTEL        = "hotel/"
	HOTELDIR     = "name/" + HOTEL
	HOTELGEODIR  = HOTELDIR + "geo/"
	HOTELRATE    = HOTELDIR + "rate"
	HOTELSEARCH  = HOTELDIR + "search"
	HOTELREC     = HOTELDIR + "rec"
	HOTELRESERVE = HOTELDIR + "reserve"
	HOTELUSER    = HOTELDIR + "user"
	HOTELPROF    = HOTELDIR + "prof"

	MEMFS          = "memfs"
	HTTP_ADDRS     = "http-addr"
	TRACING        = false
	N_RPC_SESSIONS = 10
)

var HOTELSVC = []string{
	HOTELGEODIR + sp.ANY + "/",
	HOTELRATE,
	HOTELSEARCH,
	HOTELREC,
	HOTELRESERVE,
	HOTELUSER,
	HOTELPROF,
	sp.DB + sp.ANY + "/",
}

var (
	nhotel    int
	imgSizeMB int
)

func init() {
	nh := os.Getenv("NHOTEL")
	if nh == "" {
		// Defaults to 80, same as original DSB.
		nhotel = 80
	} else {
		i, err := strconv.Atoi(nh)
		if err != nil {
			db.DFatalf("Can't parse nhotel: %v", err)
		}
		setNHotel(i)
	}
	isb := os.Getenv("HOTEL_IMG_SZ_MB")
	if isb != "" {
		i, err := strconv.Atoi(isb)
		if err != nil {
			db.DFatalf("Can't parse imgSize: %v", err)
		}
		imgSizeMB = i
	}
}

func setNHotel(nh int) {
	nhotel = nh
}

func JobDir(job string) string {
	return filepath.Join(HOTELDIR, job)
}

func JobHTTPAddrsPath(job string) string {
	return filepath.Join(JobDir(job), HTTP_ADDRS)
}

func MemFsPath(job string) string {
	return filepath.Join(JobDir(job), MEMFS)
}

func NewFsLib(uname string, npc *dialproxyclnt.DialProxyClnt) (*fslib.FsLib, error) {
	pe := proc.GetProcEnv()
	pen := proc.NewAddedProcEnv(pe)
	fsl, err := sigmaclnt.NewFsLib(pen, npc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error newfsl: %v", err)
		return nil, err
	}
	return fsl, nil
}

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) (sp.Taddrs, error) {
	ep, err := fsl.ReadEndpoint(JobHTTPAddrsPath(job))
	if err != nil {
		return nil, err
	}
	return ep.Addrs(), err
}

func InitHotelFs(fsl *fslib.FsLib, jobname string) error {
	fsl.MkDir(HOTELDIR, 0777)
	fsl.MkDir(HOTELGEODIR, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		db.DPrintf(db.ERROR, "Mkdir %v err %v\n", JobDir(jobname), err)
		return err
	}
	return nil
}

type Srv struct {
	Name string
	Args []string
	Mcpu proc.Tmcpu
}

var geo *Srv = &Srv{"hotel-geod", nil, 2000}

// XXX searchd only needs 2, but in order to make spawns work out we need to have it run with 3.
func NewHotelSvc() []*Srv {
	return []*Srv{
		&Srv{"hotel-userd", nil, 0},
		&Srv{"hotel-rated", nil, 2000},
		geo,
		&Srv{"hotel-profd", nil, 2000},
		&Srv{"hotel-searchd", nil, 3000},
		&Srv{"hotel-reserved", nil, 3000},
		&Srv{"hotel-recd", nil, 0},
		&Srv{"hotel-wwwd", nil, 3000},
	}
}

type HotelJob struct {
	*sigmaclnt.SigmaClnt
	cacheClnt       *cachegrpclnt.CachedSvcClnt
	cacheMgr        *cachegrpmgr.CacheMgr
	CacheAutoscaler *cachegrpclnt.Autoscaler
	pids            []sp.Tpid
	cache           string
	kvf             *kv.KVFleet
	job             string
}

func NewHotelJob(sc *sigmaclnt.SigmaClnt, job string, srvs []*Srv, nhotel int, cache string, cacheMcpu proc.Tmcpu, ncache int, gc bool, imgSizeMB int, ngeo int, ngeoidx int, geoSearchRadius int, geoNResults int) (*HotelJob, error) {
	// Set number of hotels before doing anything.
	setNHotel(nhotel)
	// Set the number of indexes to be used in each geo server
	geo.Args = []string{strconv.Itoa(ngeoidx), strconv.Itoa(geoSearchRadius), strconv.Itoa(geoNResults)}

	var cc *cachegrpclnt.CachedSvcClnt
	var cm *cachegrpmgr.CacheMgr
	var ca *cachegrpclnt.Autoscaler
	var err error
	var kvf *kv.KVFleet

	// Init fs.
	if err := InitHotelFs(sc.FsLib, job); err != nil {
		return nil, err
	}

	// Create a cache clnt.
	if ncache > 0 {
		switch cache {
		case "cached":
			db.DPrintf(db.ALWAYS, "Hotel running with cached")
			cm, err = cachegrpmgr.NewCacheMgr(sc, job, ncache, cacheMcpu, gc)
			if err != nil {
				db.DPrintf(db.ERROR, "Error NewCacheMgr %v", err)
				return nil, err
			}
			cc = cachegrpclnt.NewCachedSvcClnt(sc.FsLib, job)
			ca = cachegrpclnt.NewAutoscaler(cm, cc)
		case "kvd":
			db.DPrintf(db.ALWAYS, "Hotel running with kvd")
			kvf, err = kv.NewKvdFleet(sc, job, ncache, 0, cacheMcpu, "manual")
			if err != nil {
				return nil, err
			}
			err = kvf.Start()
			if err != nil {
				return nil, err
			}
		// XXX Remove
		case "memcached":
			db.DPrintf(db.ALWAYS, "Hotel running with memcached")
		default:
			db.DPrintf(db.ERROR, "Unrecognized hotel cache type: %v", cache)
		}
	}

	pids := make([]sp.Tpid, 0, len(srvs))

	for _, srv := range srvs {
		db.DPrintf(db.TEST, "Hotel spawn %v", srv.Name)
		p := proc.NewProc(srv.Name, append([]string{job, cache}, srv.Args...))
		p.AppendEnv("NHOTEL", strconv.Itoa(nhotel))
		p.AppendEnv("HOTEL_IMG_SZ_MB", strconv.Itoa(imgSizeMB))
		p.SetMcpu(srv.Mcpu)
		if err := sc.Spawn(p); err != nil {
			db.DPrintf(db.ERROR, "Error spawn proc %v: %v", p, err)
			return nil, err
		}
		if err = sc.WaitStart(p.GetPid()); err != nil {
			db.DPrintf(db.ERROR, "Error start proc %v: %v", p, err)
			return nil, err
		}
		pids = append(pids, p.GetPid())
		db.DPrintf(db.TEST, "Hotel started %v", srv.Name)
	}

	hj := &HotelJob{sc, cc, cm, ca, pids, cache, kvf, job}

	if ngeo > 1 {
		for i := 0; i < ngeo-1; i++ {
			if err := hj.AddGeoSrv(); err != nil {
				return nil, err
			}
		}
	}

	return hj, nil
}

func (hj *HotelJob) AddGeoSrv() error {
	p := proc.NewProc(geo.Name, append([]string{hj.job, hj.cache}, geo.Args...))
	p.AppendEnv("NHOTEL", strconv.Itoa(nhotel))
	p.AppendEnv("HOTEL_IMG_SZ_MB", strconv.Itoa(imgSizeMB))
	p.SetMcpu(geo.Mcpu)
	db.DPrintf(db.TEST, "Hotel spawn additional %v", geo.Name)
	db.DPrintf(db.TEST, "Hotel spawn additional %v %v", geo.Name, p)
	if err := hj.Spawn(p); err != nil {
		db.DPrintf(db.ERROR, "Error spawn proc %v: %v", p, err)
		return err
	}
	if err := hj.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.ERROR, "Error start proc %v: %v", p, err)
		return err
	}
	hj.pids = append(hj.pids, p.GetPid())
	db.DPrintf(db.TEST, "Hotel started %v", geo.Name)
	return nil
}

func (hj *HotelJob) Stop() error {
	if hj.CacheAutoscaler != nil {
		hj.CacheAutoscaler.Stop()
	}
	for _, pid := range hj.pids {
		if err := hj.Evict(pid); err != nil {
			return err
		}
		if _, err := hj.WaitExit(pid); err != nil {
			return err
		}
	}
	if hj.cacheMgr != nil {
		hj.cacheMgr.Stop()
	}
	if hj.kvf != nil {
		hj.kvf.Stop()
	}
	return nil
}

func (hj *HotelJob) StatsSrv() ([]*rpc.RPCStatsSnapshot, error) {
	if hj.cacheClnt != nil {
		return hj.cacheClnt.StatsSrvs()
	}
	db.DPrintf(db.ALWAYS, "No cacheclnt")
	return nil, nil
}
