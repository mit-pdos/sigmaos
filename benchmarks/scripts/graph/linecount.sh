#!/bin/bash
#
# linecount.sh: print a table of lines of code in the sigmaOS project,
# excluding blank lines and comments, broken down by category (system
# components, client libraries, applications, etc.).
#
# Counting rules:
#   - Go, Rust, C/C++, and protobuf files are stripped of //-style and
#     /* */-style comments; Python files of #-comments and (approximately)
#     docstrings. A line counts if anything non-whitespace remains.
#   - Generated code is excluded: *.pb.go, *.pb.h, *.pb.cc, rs/proto
#     (generated Rust protos).
#   - Go tests (*_test.go) are counted in a separate column.
#   - Vendored third-party code (apps/serverless-benchmarks) is listed but
#     flagged, and excluded from the grand total.
#   - Deployment/ops scripts (aws/, cloudlab/, docker/, k8s/, scripts/) and
#     docs are not counted.

ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
cd "$ROOT" || exit 1

# awk program: count non-blank, non-comment lines of C-style-commented
# languages (Go, Rust, C/C++, proto). Prints "<nfiles> <loc>".
CSTRIP='
FNR == 1 { inblock = 0 }
{
  line = $0; out = ""
  while (length(line) > 0) {
    if (inblock) {
      e = index(line, "*/")
      if (e == 0) { line = ""; break }
      line = substr(line, e + 2); inblock = 0
    } else {
      s = index(line, "/*"); l = index(line, "//")
      if (l > 0 && (s == 0 || l < s)) { out = out substr(line, 1, l - 1); line = "" }
      else if (s > 0) { out = out substr(line, 1, s - 1); inblock = 1; line = substr(line, s + 2) }
      else { out = out line; line = "" }
    }
  }
  if (out ~ /[^[:space:]]/) n++
}
END { print ARGC - 1, n + 0 }'

# awk program for Python: strip #-comments and (approximately) docstrings.
PYSTRIP='
FNR == 1 { indoc = 0 }
{
  line = $0
  nq = gsub(/"""/, "", line) + gsub(/\x27\x27\x27/, "", line)
  if (indoc) { if (nq % 2 == 1) indoc = 0; next }
  if (nq % 2 == 1) indoc = 1
  sub(/#.*/, "", line)
  if (line ~ /[^[:space:]]/) n++
}
END { print ARGC - 1, n + 0 }'

# sum2: sum "<nfiles> <loc>" lines from multiple awk batches
sum2() { awk '{ f += $1; n += $2 } END { print f + 0, n + 0 }'; }

# Counters. Each takes a list of paths and prints "<nfiles> <loc>".
go_code() {
	find "$@" -name '*.go' ! -name '*_test.go' ! -name '*.pb.go' -print0 2>/dev/null |
		xargs -0 -r -n 500 awk "$CSTRIP" | sum2
}
go_test() {
	find "$@" -name '*_test.go' -print0 2>/dev/null |
		xargs -0 -r -n 500 awk "$CSTRIP" | sum2
}
proto_code() {
	find "$@" -name '*.proto' -print0 2>/dev/null |
		xargs -0 -r -n 500 awk "$CSTRIP" | sum2
}
rust_code() {
	find "$@" -name '*.rs' -not -path '*/target/*' -not -path 'rs/proto/*' -print0 2>/dev/null |
		xargs -0 -r -n 500 awk "$CSTRIP" | sum2
}
cpp_code() {
	find "$@" \( -name '*.cc' -o -name '*.cpp' -o -name '*.h' -o -name '*.hpp' \) \
		! -name '*.pb.h' ! -name '*.pb.cc' \
		-not -path '*/build/*' -print0 2>/dev/null |
		xargs -0 -r -n 500 awk "$CSTRIP" | sum2
}
py_code() {
	find "$@" -name '*.py' -print0 2>/dev/null |
		xargs -0 -r -n 500 awk "$PYSTRIP" | sum2
}

# Accumulators
CAT_FILES=0 CAT_CODE=0 CAT_TEST=0
TOT_FILES=0 TOT_CODE=0 TOT_TEST=0
# Per-language accumulators (files, loc)
GO_F=0 GO_L=0 GOTEST_F=0 GOTEST_L=0 PROTO_F=0 PROTO_L=0
RS_F=0 RS_L=0 CPP_F=0 CPP_L=0 PY_F=0 PY_L=0

header() {
	printf '\n%s\n' "$1"
	printf '%-44s %7s %9s %9s %9s\n' "  component" "files" "code" "tests" "total"
	CAT_FILES=0 CAT_CODE=0 CAT_TEST=0
}

# row <label> <paths>...
# code = Go (non-test) + proto + Python (some apps/benchmarks have Python
# components); tests = *_test.go.
row() {
	local label=$1
	shift
	local cf cl tf tl pf pl yf yl
	read -r cf cl <<<"$(go_code "$@")"
	read -r pf pl <<<"$(proto_code "$@")"
	read -r yf yl <<<"$(py_code "$@")"
	read -r tf tl <<<"$(go_test "$@")"
	GO_F=$((GO_F + cf)) GO_L=$((GO_L + cl))
	PROTO_F=$((PROTO_F + pf)) PROTO_L=$((PROTO_L + pl))
	PY_F=$((PY_F + yf)) PY_L=$((PY_L + yl))
	GOTEST_F=$((GOTEST_F + tf)) GOTEST_L=$((GOTEST_L + tl))
	emit "$label" $((cf + pf + yf)) $((cl + pl + yl)) "$tf" "$tl"
}

# rowc <label> <counter> <paths>... : row for a non-Go language counter
rowc() {
	local label=$1 counter=$2
	shift 2
	local cf cl
	read -r cf cl <<<"$("$counter" "$@")"
	case "$counter" in
	rust_code) RS_F=$((RS_F + cf)) RS_L=$((RS_L + cl)) ;;
	cpp_code) CPP_F=$((CPP_F + cf)) CPP_L=$((CPP_L + cl)) ;;
	py_code) PY_F=$((PY_F + cf)) PY_L=$((PY_L + cl)) ;;
	esac
	emit "$label" "$cf" "$cl" 0 0
}

emit() { # label files code testfiles testloc
	printf '%-44s %7d %9d %9d %9d\n' "  $1" "$2" "$3" "$5" "$(($3 + $5))"
	CAT_FILES=$((CAT_FILES + $2 + $4))
	CAT_CODE=$((CAT_CODE + $3))
	CAT_TEST=$((CAT_TEST + $5))
}

subtotal() {
	printf '%-44s %7d %9d %9d %9d\n' "  -- subtotal" "$CAT_FILES" "$CAT_CODE" "$CAT_TEST" "$((CAT_CODE + CAT_TEST))"
	TOT_FILES=$((TOT_FILES + CAT_FILES))
	TOT_CODE=$((TOT_CODE + CAT_CODE))
	TOT_TEST=$((TOT_TEST + CAT_TEST))
}

echo "sigmaOS lines of code (blank lines and comments excluded)"
echo "generated code (*.pb.go, *.pb.h, *.pb.cc, rs/proto) excluded; deployment scripts not counted"

header "SigmaOS system components"
row "kernel & boot" kernel boot
row "schedulers (msched, besched, lcsched)" sched
row "naming (namesrv)" namesrv
row "server infra (sigmasrv, spproto, protsrv)" sigmasrv spproto protsrv session ctx
row "proxies (ux, s3, sigmap, wasm, db, ...)" proxy/ux proxy/s3 proxy/sigmap proxy/wasm proxy/db \
	proxy/mongo proxy/sqs proxy/ninep proxy/cpp
row "isolation & containers" container scontainer dcontainer gvisor
row "realms" realm
row "networking (net, dialproxy)" net dialproxy
row "autoscaling" autoscale
subtotal

header "SigmaOS client libraries"
row "sigmaclnt (fslib, procclnt, ...)" sigmaclnt
row "proc API" proc
row "rpc" rpc
row "getput (UX/S3 get/put reader/writer)" proxy/getput
row "fault tolerance (ft)" ft
row "shmem" shmem
subtotal

header "Protocol & shared definitions"
row "sigmap protocol" sigmap
row "api (fs, sigmaos interfaces)" api
row "9P (ninep)" ninep
row "errors & paths (serr, path)" serr path
subtotal

header "Applications"
for d in apps/*/; do
	app=$(basename "$d")
	case "$app" in
	serverless-benchmarks) continue ;; # vendored; reported below
	esac
	row "$app" "$d"
done
subtotal

header "Binaries & entrypoints"
row "cmd (proc/kernel main()s)" cmd
subtotal

header "Non-Go runtimes & libraries"
rowc "Rust (WASM boot scripts, runtime)" rust_code rs
rowc "C++ (client library, apps)" cpp_code cpp
rowc "Python (runtime, apps)" py_code python
subtotal

header "Benchmarks, tests & tools"
row "benchmarks (Go + Python scripts)" benchmarks
row "test infra" test
row "support libs (util, debug, malloc, env)" util debug malloc env
row "examples, tutorial, simulation" example tutorial simulation
subtotal

printf '\n%-44s %7d %9d %9d %9d\n' "GRAND TOTAL" "$TOT_FILES" "$TOT_CODE" "$TOT_TEST" "$((TOT_CODE + TOT_TEST))"

printf '\n%s\n' "By language"
printf '%-44s %7s %9s\n' "  language" "files" "loc"
printf '%-44s %7d %9d\n' "  Go" "$GO_F" "$GO_L"
printf '%-44s %7d %9d\n' "  Go tests" "$GOTEST_F" "$GOTEST_L"
printf '%-44s %7d %9d\n' "  Protobuf" "$PROTO_F" "$PROTO_L"
printf '%-44s %7d %9d\n' "  Rust" "$RS_F" "$RS_L"
printf '%-44s %7d %9d\n' "  C++" "$CPP_F" "$CPP_L"
printf '%-44s %7d %9d\n' "  Python" "$PY_F" "$PY_L"

read -r vf vl <<<"$(py_code apps/serverless-benchmarks)"
printf '\n%-44s %7d %9d\n' "(vendored, not counted: serverless-benchmarks)" "$vf" "$vl"
