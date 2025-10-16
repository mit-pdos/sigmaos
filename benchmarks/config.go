package benchmarks

import (
	"encoding/json"
	"fmt"
	"time"

	cossimsrv "sigmaos/apps/cossim/srv"
)

type CosSimBenchConfig struct {
	JobCfg      *cossimsrv.CosSimJobConfig `json:"job_cfg"`
	NVecToQuery int                        `json:"n_vec_to_query"`
	Durs        []time.Duration            `json:"durs"`
	MaxRPS      []int                      `json:"max_rps"`
	Scale       *ManualScalingConfig       `json:"scale"`
}

func (cfg *CosSimBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v NVecToQuery:%v Durs:%v MaxRPS:%v Scale:%v }",
		cfg.JobCfg, cfg.NVecToQuery, cfg.Durs, cfg.MaxRPS, cfg.Scale)
}

func (cfg *CosSimBenchConfig) GetJobConfig() *cossimsrv.CosSimJobConfig {
	return cfg.JobCfg
}

func (cfg *CosSimBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

//type CacheBenchConfig struct {
//	JobCfg        *cachegrpmgr.CacheJobConfig `json:"job_cfg"`
//	CPP           bool                        `json:"cpp"`
//	RunSleeper    bool                        `json:"run_sleeper"`
//	NCache        int                         `json:"n_cache"`
//	MCPU          int                         `json:"mcpu"`
//	CossimBackend bool                        `json:"cossim_backend"`
//	UseEPCache    bool                        `json:"use_epcache"`
//	DelegateInit  bool                        `json:"delegate_init"`
//	NKeys         int                         `json:"n_keys"`
//	TopNShards    int                         `json:"top_n_shards"`
//	Durs          []time.Duration             `json:"durs"`
//	MaxRPS        []int                       `json:"max_rps"`
//	PutDurs       []time.Duration             `json:"put_durs"`
//	PutMaxRPS     []int                       `json:"put_max_rps"`
//	Scale         *ManualScalingConfig        `json:"scale"`
//}

//func (cfg *CachedBenchConfig) String() string {
//	return fmt.Sprintf("&{ CPP:%v RunSleeper:%v NCache:%v MCPU:%v CossimBackend:%v UseEPCache:%v DelegateInit:%v NKeys:%v TopNShards:%v Durs:%v MaxRPS:%v PutDurs:%v PutMaxRPS:%v Scale:%v }",
//		cfg.CPP, cfg.RunSleeper, cfg.NCache, cfg.MCPU, cfg.CossimBackend, cfg.UseEPCache, cfg.DelegateInit, cfg.NKeys, cfg.TopNShards, cfg.Durs, cfg.MaxRPS, cfg.PutDurs, cfg.PutMaxRPS, cfg.Scale)
//}

//type HotelBenchConfig struct {
//	NClients        int
//	NCache          int
//	NGeo            int
//	NGeoIdx         int
//	GeoSearchRadius int
//	GeoNResults     int
//	CacheMCPU       int
//	ImgSzMB         int
//	CacheAutoscale  bool
//	UseMatch        bool
//	Durs            string
//	MaxRPS          string
//	CacheType       string
//	ScaleCache      *ManualScalingConfig
//	ScaleGeo        *ManualScalingConfig
//	ScaleCossim     *ManualScalingConfig
//}
