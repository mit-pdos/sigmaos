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
	JobCfg        *cossimsrv.CosSimJobConfig `json:"job_cfg"`
	NVecToQuery   int                        `json:"n_vec_to_query"`
	Durs          []time.Duration            `json:"durs"`
	MaxRPS        []int                      `json:"max_rps"`
	ManuallyScale *ManualScalingConfig       `json:"manually_scale"`
	Autoscale     *AutoscalingConfig         `json:"autoscale"`
}

func (cfg *CosSimBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v NVecToQuery:%v Durs:%v MaxRPS:%v ManuallyScale:%v Autoscale:%v }",
		cfg.JobCfg, cfg.NVecToQuery, cfg.Durs, cfg.MaxRPS, cfg.ManuallyScale, cfg.Autoscale)
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
	Autoscale     bool                        `json:"autoscale"`
	NKeys         int                         `json:"n_keys"`
	TopNShards    int                         `json:"top_n_shards"`
	Durs          []time.Duration             `json:"durs"`
	MaxRPS        []int                       `json:"max_rps"`
	PutDurs       []time.Duration             `json:"put_durs"`
	PutMaxRPS     []int                       `json:"put_max_rps"`
	ManuallyScale *ManualScalingConfig        `json:"manually_scale"`
	Migrate       *MigrationConfig            `json:"migrate"`
}

func (cfg *CacheBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v CPP:%v RunSleeper:%v CosSimBackend:%v UseEPCache:%v DelegateInit:%v Autoscale:%v NKeys:%v TopNShards:%v Durs:%v MaxRPS:%v PutDurs:%v PutMaxRPS:%v ManuallyScale:%v Migrate:%v }",
		cfg.JobCfg, cfg.CPP, cfg.RunSleeper, cfg.CosSimBackend, cfg.UseEPCache, cfg.DelegateInit, cfg.Autoscale, cfg.NKeys, cfg.TopNShards, cfg.Durs, cfg.MaxRPS, cfg.PutDurs, cfg.PutMaxRPS, cfg.ManuallyScale, cfg.Migrate)
}

func (cfg *CacheBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type HotelBenchConfig struct {
	JobCfg          *hotel.HotelJobConfig `json:"job_cfg"`
	Durs            []time.Duration       `json:"durs"`
	MaxRPS          []int                 `json:"max_rps"`
	ScaleGeo        *ManualScalingConfig  `json:"scale_geo"`
	MatchUseCaching bool                  `json:"match_use_caching"`
	CachedUserFrac  int64                 `json:"cached_user_frac"`
	CacheBenchCfg   *CacheBenchConfig     `json:"cache_bench_cfg"`
	CosSimBenchCfg  *CosSimBenchConfig    `json:"cossim_bench_cfg"`
}

func (cfg *HotelBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v Durs:%v MaxRPS:%v ScaleGeo:%v MatchUseCaching:%v CachedUserFrac:%v CacheBenchCfg:%v CosSimBenchCfg:%v }",
		cfg.JobCfg, cfg.Durs, cfg.MaxRPS, cfg.ScaleGeo, cfg.MatchUseCaching, cfg.CachedUserFrac, cfg.CacheBenchCfg, cfg.CosSimBenchCfg)
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
