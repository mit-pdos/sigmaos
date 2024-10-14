package hotel

import (
	"os"
	"path/filepath"
	"strconv"

	"sigmaos/cachedsvc"
	"sigmaos/cachedsvcclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kv"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt"
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
	HOTELGEODIR + "~any/",
	HOTELRATE,
	HOTELSEARCH,
	HOTELREC,
	HOTELRESERVE,
	HOTELUSER,
	HOTELPROF,
	sp.DB + "~any/",
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

func NewFsLibs(uname string, npc *netproxyclnt.NetProxyClnt) ([]*fslib.FsLib, error) {
	pe := proc.GetProcEnv()
	fsls := make([]*fslib.FsLib, 0, N_RPC_SESSIONS)
	for i := 0; i < N_RPC_SESSIONS; i++ {
		pen := proc.NewAddedProcEnv(pe)
		fsl, err := sigmaclnt.NewFsLib(pen, npc)
		if err != nil {
			db.DPrintf(db.ERROR, "Error newfsl: %v", err)
			return nil, err
		}
		fsls = append(fsls, fsl)
	}
	return fsls, nil
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

var geo Srv = Srv{"hotel-geod", nil, 2000}

// XXX searchd only needs 2, but in order to make spawns work out we need to have it run with 3.
func NewHotelSvc(public bool) []Srv {
	return []Srv{
		Srv{"hotel-userd", nil, 0},
		Srv{"hotel-rated", nil, 2000},
		geo,
		Srv{"hotel-profd", nil, 2000},
		Srv{"hotel-searchd", nil, 3000},
		Srv{"hotel-reserved", nil, 3000},
		Srv{"hotel-recd", nil, 0},
		Srv{"hotel-wwwd", []string{strconv.FormatBool(public)}, 3000},
	}
}

//var ncores = []int{0, 1,
//	1, 1, 3,
//	3, 0, 2}

//var ncores = []int{0, 2,
//	2, 2, 3,
//	3, 0, 2}

type HotelJob struct {
	*sigmaclnt.SigmaClnt
	cacheClnt       *cachedsvcclnt.CachedSvcClnt
	cacheMgr        *cachedsvc.CacheMgr
	CacheAutoscaler *cachedsvcclnt.Autoscaler
	pids            []sp.Tpid
	cache           string
	kvf             *kv.KVFleet
	job             string
}

func NewHotelJob(sc *sigmaclnt.SigmaClnt, job string, srvs []Srv, nhotel int, cache string, cacheMcpu proc.Tmcpu, nsrv int, gc bool, imgSizeMB int) (*HotelJob, error) {
	// Set number of hotels before doing anything.
	setNHotel(nhotel)

	var cc *cachedsvcclnt.CachedSvcClnt
	var cm *cachedsvc.CacheMgr
	var ca *cachedsvcclnt.Autoscaler
	var err error
	var kvf *kv.KVFleet

	// Init fs.
	if err := InitHotelFs(sc.FsLib, job); err != nil {
		return nil, err
	}

	// Create a cache clnt.
	if nsrv > 0 {
		switch cache {
		case "cached":
			db.DPrintf(db.ALWAYS, "Hotel running with cached")
			cm, err = cachedsvc.NewCacheMgr(sc, job, nsrv, cacheMcpu, gc)
			if err != nil {
				db.DPrintf(db.ERROR, "Error NewCacheMgr %v", err)
				return nil, err
			}
			cc = cachedsvcclnt.NewCachedSvcClnt([]*fslib.FsLib{sc.FsLib}, job)
			ca = cachedsvcclnt.NewAutoscaler(cm, cc)
		case "kvd":
			db.DPrintf(db.ALWAYS, "Hotel running with kvd")
			kvf, err = kv.NewKvdFleet(sc, job, 0, nsrv, 0, 0, cacheMcpu, "0", "manual")
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

	return &HotelJob{sc, cc, cm, ca, pids, cache, kvf, job}, nil
}

func (hj *HotelJob) AddGeoSrv() error {
	p := proc.NewProc(geo.Name, append([]string{hj.job, hj.cache}, geo.Args...))
	p.AppendEnv("NHOTEL", strconv.Itoa(nhotel))
	p.AppendEnv("HOTEL_IMG_SZ_MB", strconv.Itoa(imgSizeMB))
	p.SetMcpu(geo.Mcpu)
	db.DPrintf(db.TEST, "Hotel spawn additional %v", geo.Name)
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
