module sigmaos

go 1.21

toolchain go1.21.3

require (
	github.com/aws/aws-lambda-go v1.31.0
	github.com/aws/aws-sdk-go v1.44.107
	github.com/aws/aws-sdk-go-v2 v1.16.2
	github.com/aws/aws-sdk-go-v2/config v1.15.3
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.4
	github.com/aws/aws-sdk-go-v2/service/s3 v1.26.4
	github.com/dustin/go-humanize v1.0.1
	github.com/go-redis/redis/v8 v8.11.5
	github.com/go-sql-driver/mysql v1.6.0
	github.com/hailocab/go-geoindex v0.0.0-20160127134810-64631bfe9711
	github.com/harlow/go-micro-services v0.0.0-20210513051106-5e6a90aabee6
	github.com/klauspost/readahead v1.4.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/montanaflynn/stats v0.6.6
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/stretchr/testify v1.8.4
	github.com/thanhpk/randstr v1.0.4
	go.uber.org/zap v1.27.0
	golang.org/x/sys v0.20.0
	gonum.org/v1/gonum v0.12.0
	google.golang.org/protobuf v1.34.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ArielSzekely/etcd/server/v3 v3.5.14-0.20240506164909-b4ba8567721e
	github.com/aws/aws-sdk-go-v2/credentials v1.11.2
	github.com/bradfitz/gomemcache v0.0.0-20230124162541-5f7a7d875746
	github.com/docker/docker v23.0.1+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/protobuf v1.5.4
	github.com/hanwen/go-fuse/v2 v2.5.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/mit-pdos/go-geoindex v0.0.0-20230316114931-aab59857d7c8
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/opencontainers/runc v1.1.4
	github.com/opencontainers/selinux v1.11.0
	github.com/shirou/gopsutil v2.21.11+incompatible
	go.etcd.io/etcd/client/pkg/v3 v3.5.13
	go.etcd.io/etcd/client/v3 v3.5.13
	go.etcd.io/etcd/raft/v3 v3.5.13
	go.etcd.io/etcd/server/v3 v3.5.13
	go.mongodb.org/mongo-driver v1.12.1
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.40.0
	go.opentelemetry.io/otel v1.20.0
	go.opentelemetry.io/otel/exporters/jaeger v1.14.0
	go.opentelemetry.io/otel/sdk v1.20.0
	go.opentelemetry.io/otel/trace v1.20.0
	golang.org/x/exp v0.0.0-20240404231335-c0f41cb1a7a0
	google.golang.org/grpc v1.59.0
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	kernel.org/pub/linux/libs/security/libcap/cap v1.2.67
)

require (
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.3 // indirect
	github.com/aws/smithy-go v1.11.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.19.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.53.0 // indirect
	github.com/prometheus/procfs v0.14.0 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xiang90/probing v0.0.0-20221125231312-a49e3df8f510 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.etcd.io/etcd/api/v3 v3.5.13 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.13 // indirect
	go.opentelemetry.io/otel/metric v1.20.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
	google.golang.org/genproto v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	gotest.tools/v3 v3.4.0 // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.67 // indirect
)
