package benchmarks

import (
	"encoding/json"
	"fmt"
	"time"

	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/hotel"
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

type CacheBenchConfig struct {
	JobCfg        *cachegrpmgr.CacheJobConfig `json:"job_cfg"`
	CPP           bool                        `json:"cpp"`
	RunSleeper    bool                        `json:"run_sleeper"`
	CosSimBackend bool                        `json:"cossim_backend"`
	UseEPCache    bool                        `json:"use_epcache"`
	DelegateInit  bool                        `json:"delegate_init"`
	NKeys         int                         `json:"n_keys"`
	TopNShards    int                         `json:"top_n_shards"`
	Durs          []time.Duration             `json:"durs"`
	MaxRPS        []int                       `json:"max_rps"`
	PutDurs       []time.Duration             `json:"put_durs"`
	PutMaxRPS     []int                       `json:"put_max_rps"`
	Scale         *ManualScalingConfig        `json:"scale"`
}

func (cfg *CacheBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v CPP:%v RunSleeper:%v CosSimBackend:%v UseEPCache:%v DelegateInit:%v NKeys:%v TopNShards:%v Durs:%v MaxRPS:%v PutDurs:%v PutMaxRPS:%v Scale:%v }",
		cfg.JobCfg, cfg.CPP, cfg.RunSleeper, cfg.CosSimBackend, cfg.UseEPCache, cfg.DelegateInit, cfg.NKeys, cfg.TopNShards, cfg.Durs, cfg.MaxRPS, cfg.PutDurs, cfg.PutMaxRPS, cfg.Scale)
}

func (cfg *CacheBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type HotelBenchConfig struct {
	JobCfg   *hotel.HotelJobConfig `json:"job_cfg"`
	Durs     []time.Duration       `json:"durs"`
	MaxRPS   []int                 `json:"max_rps"`
	ScaleGeo *ManualScalingConfig  `json:"scale_geo"`
}

func (cfg *HotelBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v Durs:%v MaxRPS:%v ScaleGeo:%v }",
		cfg.JobCfg, cfg.Durs, cfg.MaxRPS, cfg.ScaleGeo)
}

func (cfg *HotelBenchConfig) GetJobConfig() *hotel.HotelJobConfig {
	return cfg.JobCfg
}

func (cfg *HotelBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
