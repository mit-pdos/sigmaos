package cpp_test

import (
	"bytes"
	"encoding/binary"
	"flag"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	wasmer "github.com/wasmerio/wasmer-go/wasmer"
	"google.golang.org/protobuf/proto"

	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cacheproto "sigmaos/apps/cache/proto"
	db "sigmaos/debug"
)

var wasmScript string
var cossimBootScript string

func init() {
	flag.StringVar(&wasmScript, "wasm_script", "/home/arielck/sigmaos/rs/wasm/hello-wasm/target/wasm32-unknown-unknown/release/hello_wasm.wasm", "path to WASM script")
	flag.StringVar(&cossimBootScript, "cossim_boot_script", "/home/arielck/sigmaos/rs/wasm/cossim-boot/target/wasm32-unknown-unknown/release/cossim_boot.wasm", "path to WASM script")
}

func TestCompile(t *testing.T) {
}

func TestHelloWorld(t *testing.T) {
	wasmScript, err := os.ReadFile(wasmScript)
	if !assert.Nil(t, err, "Err read wasm script: %v", err) {
		return
	}
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	start := time.Now()
	// Compiles the module
	module, err := wasmer.NewModule(store, wasmScript)
	if !assert.Nil(t, err, "Err compile wasm module: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Wasm module compilation (%vB) latency: %v", len(wasmScript), time.Since(start))

	// Instantiates the module
	importObject := wasmer.NewImportObject()
	instance, err := wasmer.NewInstance(module, importObject)
	if !assert.Nil(t, err, "Err instantiate wasm module: %v", err) {
		return
	}

	// Gets the `add` exported function from the WebAssembly instance.
	add, err := instance.Exports.GetFunction("add")
	if !assert.Nil(t, err, "Err get wasm function: %v", err) {
		return
	}

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := add(5, 37)
	if !assert.Nil(t, err, "Err call wasm function: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Result: %v", result) // 42!
}

func TestLatency(t *testing.T) {
	const (
		NTRIAL = 10
	)
	wasmScript, err := os.ReadFile(wasmScript)
	if !assert.Nil(t, err, "Err read wasm script: %v", err) {
		return
	}
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	var module *wasmer.Module

	durs := make([]time.Duration, 0, NTRIAL)
	start := time.Now()
	for i := 0; i < NTRIAL; i++ {
		start := time.Now()
		// Compiles the module
		module, err = wasmer.NewModule(store, wasmScript)
		if !assert.Nil(t, err, "Err compile wasm module: %v", err) {
			return
		}
		durs = append(durs, time.Since(start))
	}
	db.DPrintf(db.TEST, "Wasm module compilation (%vB) latency:\n\tAvg: %v\n\tEach:%v", len(wasmScript), time.Since(start)/NTRIAL, durs)

	var instance *wasmer.Instance

	durs = make([]time.Duration, 0, NTRIAL)
	start = time.Now()
	for i := 0; i < NTRIAL; i++ {
		start := time.Now()
		// Instantiates the module
		importObject := wasmer.NewImportObject()
		instance, err = wasmer.NewInstance(module, importObject)
		if !assert.Nil(t, err, "Err instantiate wasm module: %v", err) {
			return
		}
		durs = append(durs, time.Since(start))
	}
	db.DPrintf(db.TEST, "Wasm module instantiation (%vB) latency:\n\tAvg: %v\n\tEach:%v", len(wasmScript), time.Since(start)/NTRIAL, durs)

	var add wasmer.NativeFunction

	durs = make([]time.Duration, 0, NTRIAL)
	start = time.Now()
	for i := 0; i < NTRIAL; i++ {
		start := time.Now()
		// Gets the `add` exported function from the WebAssembly instance.
		add, err = instance.Exports.GetFunction("add")
		if !assert.Nil(t, err, "Err get wasm function: %v", err) {
			return
		}
		durs = append(durs, time.Since(start))
	}
	db.DPrintf(db.TEST, "Wasm function retrieval latency:\n\tAvg: %v\n\tEach:%v", time.Since(start)/NTRIAL, durs)

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := add(5, 37)
	if !assert.Nil(t, err, "Err call wasm function: %v", err) {
		return
	}

	assert.Equal(t, result.(int32), int32(42))
}

var logged bool
var loggedVal int32

func log(v int32) {
	logged = true
	loggedVal = v
	db.DPrintf(db.TEST, "Logged from wasm: %v", v)
}

func TestHostFunction(t *testing.T) {
	wasmScript, err := os.ReadFile(wasmScript)
	if !assert.Nil(t, err, "Err read wasm script: %v", err) {
		return
	}
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	logHostFn := wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			log(args[0].I32())
			return []wasmer.Value{}, nil
		},
	)

	start := time.Now()
	// Compiles the module
	module, err := wasmer.NewModule(store, wasmScript)
	if !assert.Nil(t, err, "Err compile wasm module: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Wasm module compilation (%vB) latency: %v", len(wasmScript), time.Since(start))

	// Instantiates the module
	importObject := wasmer.NewImportObject()
	importObject.Register(
		"sigmaos_host",
		map[string]wasmer.IntoExtern{
			"log_int": logHostFn,
		},
	)
	instance, err := wasmer.NewInstance(module, importObject)
	if !assert.Nil(t, err, "Err instantiate wasm module: %v", err) {
		return
	}

	// Gets the `add` exported function from the WebAssembly instance.
	add_and_log, err := instance.Exports.GetFunction("add_and_log")
	if !assert.Nil(t, err, "Err get wasm function: %v", err) {
		return
	}

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := add_and_log(5, 37)
	if !assert.Nil(t, err, "Err call wasm function: %v", err) {
		return
	}

	if !assert.True(t, logged, "log host function never called") {
		return
	}

	db.DPrintf(db.TEST, "Result: %v", result) // 42!
}

func TestMemory(t *testing.T) {
	const (
		ALLOC_SZ = 1024
	)
	wasmScript, err := os.ReadFile(wasmScript)
	if !assert.Nil(t, err, "Err read wasm script: %v", err) {
		return
	}
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	logHostFn := wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			log(args[0].I32())
			return []wasmer.Value{}, nil
		},
	)

	start := time.Now()
	// Compiles the module
	module, err := wasmer.NewModule(store, wasmScript)
	if !assert.Nil(t, err, "Err compile wasm module: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Wasm module compilation (%vB) latency: %v", len(wasmScript), time.Since(start))

	// Instantiates the module
	importObject := wasmer.NewImportObject()
	importObject.Register(
		"sigmaos_host",
		map[string]wasmer.IntoExtern{
			"log_int": logHostFn,
		},
	)
	instance, err := wasmer.NewInstance(module, importObject)
	if !assert.Nil(t, err, "Err instantiate wasm module: %v", err) {
		return
	}

	allocFn, err := instance.Exports.GetFunction("allocate")
	if !assert.Nil(t, err, "Err get allocate wasm function: %v", err) {
		return
	}
	allocateResult, err := allocFn(ALLOC_SZ)
	if !assert.Nil(t, err, "Err allocate wasm mem: %v", err) {
		return
	}
	inputPointer := allocateResult.(int32)
	db.DPrintf(db.TEST, "Alloc result: %v", allocateResult)
	mem, err := instance.Exports.GetMemory("memory")
	if !assert.Nil(t, err, "Err get wasm mem: %v", err) {
		return
	}
	buf := mem.Data()[inputPointer : inputPointer+ALLOC_SZ]
	buf[0] = 'A'

	// Gets the `add` exported function from the WebAssembly instance.
	add_and_log_with_mem, err := instance.Exports.GetFunction("add_and_log_with_mem")
	if !assert.Nil(t, err, "Err get wasm function: %v", err) {
		return
	}

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := add_and_log_with_mem(5, 37, allocateResult, ALLOC_SZ)
	if !assert.Nil(t, err, "Err call wasm function: %v", err) {
		return
	}

	if !assert.True(t, logged, "log host function never called") {
		return
	}

	if !assert.Equal(t, int32(buf[0]), loggedVal, "WASM buffer read didn't work") {
		return
	}

	if !assert.Equal(t, buf[1], buf[0]+1, "WASM buffer write didn't work") {
		return
	}

	db.DPrintf(db.TEST, "Result: %v", result) // 42!
}

func TestCosSimBoot(t *testing.T) {
	const (
		BUF_SZ        = 655360
		N_SRV  uint32 = 2
		N_KEYS uint64 = 120
	)
	warmupScript, err := os.ReadFile(wasmScript)
	if !assert.Nil(t, err, "Err read wasm script: %v", err) {
		return
	}
	bootScript, err := os.ReadFile(cossimBootScript)
	if !assert.Nil(t, err, "Err read wasm script: %v", err) {
		return
	}
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	vecKeys := make([]string, N_KEYS)
	for i := range vecKeys {
		vecKeys[i] = strconv.Itoa(i)
	}
	cacheMultiGetReqs := cachegrpclnt.NewMultiGetReqs(vecKeys, int(N_SRV), cache.NSHARD)
	var buf []byte
	sendRPCFn := wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I64, wasmer.I64), wasmer.NewValueTypes()),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			rpcIdx := args[0].I64()
			l := args[1].I64()
			cacheMultiGet := &cacheproto.CacheMultiGetReq{}
			if err := proto.Unmarshal(buf[:l], cacheMultiGet); !assert.Nil(t, err, "Err unmarshal MultiGet: %v", err) {
				return []wasmer.Value{}, nil
			}
			// Get the "ground truth" request
			groundTruthReq := cacheMultiGetReqs[int(rpcIdx)]
			// Check that the number of gets matches
			if !assert.Equal(t, len(groundTruthReq.Gets), len(cacheMultiGet.Gets), "Mismatched number of get requests for rpcIdx: %v", rpcIdx) {
				return []wasmer.Value{}, nil
			}
			for i := range groundTruthReq.Gets {
				// Check that the key matches
				if !assert.Equal(t, groundTruthReq.Gets[i].Key, cacheMultiGet.Gets[i].Key, "Mismatched key for rpcIdx: %v getIdx: %v", rpcIdx, i) {
					return []wasmer.Value{}, nil
				}
				// Check that the shard matches
				// Check that the key matches
				if !assert.Equal(t, groundTruthReq.Gets[i].Shard, cacheMultiGet.Gets[i].Shard, "Mismatched shard for rpcIdx: %v getIdx: %v", rpcIdx, i) {
					return []wasmer.Value{}, nil
				}
			}
			return []wasmer.Value{}, nil
		},
	)

	// Warm up wasmer
	if _, err := wasmer.NewModule(store, warmupScript); !assert.Nil(t, err, "Err compile wasm module: %v", err) {
		return
	}

	start := time.Now()
	// Compiles the module
	module, err := wasmer.NewModule(store, bootScript)
	if !assert.Nil(t, err, "Err compile wasm module: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "Wasm module compilation (%vB) latency: %v", len(bootScript), time.Since(start))

	// Instantiates the module
	importObject := wasmer.NewImportObject()
	importObject.Register(
		"sigmaos_host",
		map[string]wasmer.IntoExtern{
			"send_rpc": sendRPCFn,
		},
	)
	instance, err := wasmer.NewInstance(module, importObject)
	if !assert.Nil(t, err, "Err instantiate wasm module: %v", err) {
		return
	}

	allocFn, err := instance.Exports.GetFunction("allocate")
	if !assert.Nil(t, err, "Err get allocate wasm function: %v", err) {
		return
	}
	wasmBufPtr, err := allocFn(BUF_SZ)
	if !assert.Nil(t, err, "Err allocate wasm mem: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "WASM-allocated buffer address: %v", wasmBufPtr)
	mem, err := instance.Exports.GetMemory("memory")
	if !assert.Nil(t, err, "Err get wasm mem: %v", err) {
		return
	}
	buf = mem.Data()[wasmBufPtr.(int32) : wasmBufPtr.(int32)+BUF_SZ]
	// Gets the `boot` exported function from the WebAssembly instance.
	boot, err := instance.Exports.GetFunction("boot")
	if !assert.Nil(t, err, "Err get wasm function: %v", err) {
		return
	}
	// Write the input arguments to the boot script
	inputBuf := bytes.NewBuffer(make([]byte, 0, 4))
	if err := binary.Write(inputBuf, binary.LittleEndian, N_SRV); !assert.Nil(t, err, "Err write input to boot script: %v", err) {
		return
	}
	if err := binary.Write(inputBuf, binary.LittleEndian, N_KEYS); !assert.Nil(t, err, "Err write input to boot script: %v", err) {
		return
	}
	copy(buf, inputBuf.Bytes())
	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	if _, err := boot(wasmBufPtr, BUF_SZ); !assert.Nil(t, err, "Err call wasm function: %v", err) {
		return
	}
}
