package coord

import (
	"encoding/binary"
	"testing"

	"sigmaos/apps/mr"
	mrapi "sigmaos/apps/mr/mr"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// decodeArgs reverses wasmer.EncodeArgs: n u32-LE lengths, then the bodies.
func decodeArgs(t *testing.T, b []byte, nstr int) []string {
	t.Helper()
	strs := make([]string, 0, nstr)
	off := 4 * nstr
	for i := 0; i < nstr; i++ {
		l := int(binary.LittleEndian.Uint32(b[4*i : 4*i+4]))
		strs = append(strs, string(b[off:off+l]))
		off += l
	}
	return strs
}

// The reducer's manifest must match what rs/wasm/mr_reducer_boot expects: a
// u32-LE shard count, then 4 strings per shard (typ, kid, a, b), in bin order —
// bin order is what makes rpcIdx equal the shard index Reducer.readFile asks
// for.
func TestReducerBootInput(t *testing.T) {
	bin := mr.Bin{
		{File: "name/ux/kid1/mr-intermediate/job/r-0-abc"},
		{File: "name/ux/kid2/mr-intermediate/job/r-0-def"},
		{File: "name/s3/~local/9ps3/mr-intermediate/job/r-0-ghi"},
	}
	input, err := reducerBootInput(bin)
	if err != nil {
		t.Fatalf("reducerBootInput: %v", err)
	}
	if n := binary.LittleEndian.Uint32(input[0:4]); int(n) != len(bin) {
		t.Fatalf("shard count %d want %d", n, len(bin))
	}
	strs := decodeArgs(t, input[4:], 4*len(bin))
	want := []string{
		"ux", "kid1", "mr-intermediate/job/r-0-abc", "",
		"ux", "kid2", "mr-intermediate/job/r-0-def", "",
		// An S3 shard keeps the ~local Mapper.outputBin left in place, which
		// resolves to the reducer's own kernel in the cosandbox.
		"s3", "~local", "9ps3", "mr-intermediate/job/r-0-ghi",
	}
	if len(strs) != len(want) {
		t.Fatalf("got %d strings want %d: %v", len(strs), len(want), strs)
	}
	for i := range want {
		if strs[i] != want[i] {
			t.Errorf("arg %d: got %q want %q", i, strs[i], want[i])
		}
	}
}

func TestReducerShmemMB(t *testing.T) {
	// The job description's setting wins outright — including when it is
	// smaller than what the mappers wrote, since it is the operator's call.
	if mb := reducerShmemMB(64, 10, 100*int64(sp.MBYTE), 1); mb != proc.Tmem(64) {
		t.Errorf("reducerShmemMB(cfg 64) = %v want 64", mb)
	}
	// Unset: 100 MB of intermediate output over 4 reducers is 25 MB each, plus
	// framing, plus the margin — sized to the requirement, not a multiple of it.
	// 25MB of shards + 10 replies of framing rounds up to 26MB.
	if mb := reducerShmemMB(0, 10, 100*int64(sp.MBYTE), 4); mb != 26+SHMEM_MARGIN_MB {
		t.Errorf("reducerShmemMB = %v want %v", mb, 26+SHMEM_MARGIN_MB)
	}
	// Unset with a tiny (or unknown, e.g. after a coordinator restart)
	// intermediate size still has to leave the allocator room for reply framing.
	if mb := reducerShmemMB(0, 0, 0, 1); mb != SHMEM_MARGIN_MB {
		t.Errorf("reducerShmemMB(0) = %v want %v", mb, SHMEM_MARGIN_MB)
	}
	// nreduce is a divisor: a bogus value must not panic.
	if mb := reducerShmemMB(0, 1, int64(sp.MBYTE), 0); mb != 2+SHMEM_MARGIN_MB {
		t.Errorf("reducerShmemMB(1MB, 0) = %v want %v", mb, 2+SHMEM_MARGIN_MB)
	}
}

// The mapper's segment is sized from the job's bin size, which bounds a bin's
// data (mr.NewBins closes a bin before it would exceed it).
func TestMapperShmemMB(t *testing.T) {
	// 3MB of bin data in 3 splits: the data, plus a tail probe and reply
	// framing per split, plus the margin. Emphatically not a multiple of binsz:
	// the segment is reserved from the node's /dev/shm and every concurrently
	// running mapper on the node holds one.
	if mb := mapperShmemMB(3*int(sp.MBYTE), 3, 1024*1024, 4096); mb != 4+SHMEM_MARGIN_MB {
		t.Errorf("mapperShmemMB = %v want %v", mb, 4+SHMEM_MARGIN_MB)
	}
	if mb := mapperShmemMB(1024, 1, 1024*1024, 4096); mb != 1+SHMEM_MARGIN_MB {
		t.Errorf("mapperShmemMB(1KB) = %v want %v", mb, 1+SHMEM_MARGIN_MB)
	}
	// The real benchmark shape: a 130MiB bin of 12 splits used to ask for
	// 260MB, of which ~120MB was ever touched.
	if mb := mapperShmemMB(136314880, 12, 2097152, 0); mb > 140 {
		t.Errorf("mapperShmemMB(130MiB bin) = %vMB, want it sized to the ~130MiB requirement", mb)
	}
}

// The mapper's manifest and its GetPutReader must agree on the read window, or
// the mapper maps corrupted split boundaries; both go through
// mrapi.SplitReadWindow, so pin that this is the window the manifest carries.
func TestMapperBootInputWindow(t *testing.T) {
	const linesz, probesz = 32768, 4096
	s := mrapi.Split{File: "name/ux/~local/wiki/f0", Offset: 1024, Length: 2048}
	input, err := mapperBootInput(mr.Bin{s}, linesz, probesz)
	if err != nil {
		t.Fatalf("mapperBootInput: %v", err)
	}
	strs := decodeArgs(t, input[4:], 1+5*1)
	off, body, probe := mrapi.SplitReadWindow(&s, linesz, probesz)
	if strs[4] != "1023" || strs[4] != itoa(int64(off)) {
		t.Errorf("off %q want %v (one byte early, to detect a partial first line)", strs[4], off)
	}
	if want := itoa(int64(body) + int64(probe)); strs[5] != want {
		t.Errorf("cnt %q want %q", strs[5], want)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
