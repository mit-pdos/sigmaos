package benchmarks

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/etcd"
	"sigmaos/apps/hotel"
	imgrec_py "sigmaos/apps/imgrec/py"
	imgrec_wasm "sigmaos/apps/imgrec/wasm"
	"sigmaos/apps/imgresize"
	"sigmaos/apps/memcached"
	"sigmaos/apps/mr"
	"sigmaos/proc"
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
	UseCoSandbox  bool                        `json:"use_co_sandbox"`
	Autoscale     bool                        `json:"autoscale"`
	Shmem         bool                        `json:"shmem"`
	NKeys         int                         `json:"n_keys"`
	ValSize       int                         `json:"val_size"`
	TopNShards    int                         `json:"top_n_shards"`
	Durs          []time.Duration             `json:"durs"`
	MaxRPS        []int                       `json:"max_rps"`
	PutDurs       []time.Duration             `json:"put_durs"`
	PutMaxRPS     []int                       `json:"put_max_rps"`
	ManuallyScale *ManualScalingConfig        `json:"manually_scale"`
	Migrate       *MigrationConfig            `json:"migrate"`
}

func (cfg *CacheBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v CPP:%v RunSleeper:%v CosSimBackend:%v UseEPCache:%v UseCoSandbox:%v Autoscale:%v Shmem:%v NKeys:%v ValSize:%v TopNShards:%v Durs:%v MaxRPS:%v PutDurs:%v PutMaxRPS:%v ManuallyScale:%v Migrate:%v }",
		cfg.JobCfg, cfg.CPP, cfg.RunSleeper, cfg.CosSimBackend, cfg.UseEPCache, cfg.UseCoSandbox, cfg.Autoscale, cfg.Shmem, cfg.NKeys, cfg.ValSize, cfg.TopNShards, cfg.Durs, cfg.MaxRPS, cfg.PutDurs, cfg.PutMaxRPS, cfg.ManuallyScale, cfg.Migrate)
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

type MRBenchConfig struct {
	App            string    `json:"app"`             // Name of the MR job description json file
	MemReq         proc.Tmem `json:"mem_req"`         // Amount of memory (in MB) required by each mapper/reducer
	UseGetPut      bool      `json:"use_getput"`      // Mappers use the UX/S3 proxy Get/Put client API
	UseCosandboxes bool      `json:"use_cosandboxes"` // Cosandboxes pre-fetch mapper input splits (requires UseGetPut)
	JobCfg         *mr.Job   `json:"job_cfg"`         // MR job description
}

// NewMRBenchConfig creates an MR benchmark config, reading the MR job
// description named app from jobDir on the local file system. useGetPut and
// useCosandboxes override the corresponding job-description fields, so one
// job description serves all variants.
func NewMRBenchConfig(jobDir, app string, memReq proc.Tmem, useGetPut, useCosandboxes bool) (*MRBenchConfig, error) {
	jobCfg, err := mr.ReadJobConfig(filepath.Join(jobDir, app))
	if err != nil {
		return nil, err
	}
	if useCosandboxes && !useGetPut {
		return nil, fmt.Errorf("MRBenchConfig %v: useCosandboxes requires useGetPut", app)
	}
	jobCfg.UseGetPut = useGetPut
	jobCfg.UseCosandboxes = useCosandboxes
	return &MRBenchConfig{
		App:            app,
		MemReq:         memReq,
		UseGetPut:      useGetPut,
		UseCosandboxes: useCosandboxes,
		JobCfg:         jobCfg,
	}, nil
}

func (cfg *MRBenchConfig) String() string {
	return fmt.Sprintf("&{ App:%v MemReq:%v JobCfg:%v }", cfg.App, cfg.MemReq, cfg.JobCfg)
}

func (cfg *MRBenchConfig) GetJobConfig() *mr.Job {
	return cfg.JobCfg
}

func (cfg *MRBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type ImgBenchConfig struct {
	JobCfg         *imgresize.ImgdJobConfig `json:"job_cfg"`
	InputPath      string                   `json:"input_path"`
	NTasks         int                      `json:"n_tasks"`
	NInputsPerTask int                      `json:"n_inputs_per_task"`
	Durs           []time.Duration          `json:"durs"`
	MaxRPS         []int                    `json:"max_rps"`
}

func (cfg *ImgBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v InputPath:%v NTasks:%v NInputsPerTask:%v Durs:%v MaxRPS:%v }",
		cfg.JobCfg, cfg.InputPath, cfg.NTasks, cfg.NInputsPerTask, cfg.Durs, cfg.MaxRPS)
}

func (cfg *ImgBenchConfig) GetJobConfig() *imgresize.ImgdJobConfig {
	return cfg.JobCfg
}

func (cfg *ImgBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type EtcdBenchConfig struct {
	JobCfg *etcd.EtcdJobConfig `json:"job_cfg"`
}

func (cfg *EtcdBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v }", cfg.JobCfg)
}

func (cfg *EtcdBenchConfig) GetJobConfig() *etcd.EtcdJobConfig {
	return cfg.JobCfg
}

func (cfg *EtcdBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type MemcachedBenchConfig struct {
	JobCfg *memcached.MemcachedJobConfig `json:"job_cfg"`
	Cache  bool                          `json:"cache"`
}

func (cfg *MemcachedBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v Cache:%v }", cfg.JobCfg, cfg.Cache)
}

func (cfg *MemcachedBenchConfig) GetJobConfig() *memcached.MemcachedJobConfig {
	return cfg.JobCfg
}

func (cfg *MemcachedBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type ImgrecPyBenchConfig struct {
	JobCfg *imgrec_py.ImgrecPyJobConfig `json:"job_cfg"`
}

func (cfg *ImgrecPyBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v }", cfg.JobCfg)
}

func (cfg *ImgrecPyBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type ImgrecWASMBenchConfig struct {
	JobCfg *imgrec_wasm.ImgrecWASMJobConfig `json:"job_cfg"`
}

func (cfg *ImgrecWASMBenchConfig) String() string {
	return fmt.Sprintf("&{ JobCfg:%v }", cfg.JobCfg)
}

func (cfg *ImgrecWASMBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type StartLatencyBenchConfig struct {
	App string `json:"app"`
}

func (cfg *StartLatencyBenchConfig) String() string {
	return fmt.Sprintf("&{ App:%v }", cfg.App)
}

func (cfg *StartLatencyBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type SebsBenchConfig struct {
	Benchmark    string     `json:"benchmark"`
	Event        string     `json:"event"`
	Kid          string     `json:"kid"`
	ShmemMB      proc.Tmem  `json:"shmem_mb"`
	Mcpu         proc.Tmcpu `json:"mcpu"`
	UseCoSandbox bool       `json:"use_co_sandbox"`
	AsyncFetch   bool       `json:"async_fetch"`
	Uncompressed bool       `json:"uncompressed"`
}

func (cfg *SebsBenchConfig) String() string {
	return fmt.Sprintf("&{ Benchmark:%v Kid:%v ShmemMB:%v Mcpu:%v UseCoSandbox:%v AsyncFetch:%v Uncompressed:%v }",
		cfg.Benchmark, cfg.Kid, cfg.ShmemMB, cfg.Mcpu, cfg.UseCoSandbox, cfg.AsyncFetch, cfg.Uncompressed)
}

func (cfg *SebsBenchConfig) Marshal() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
