package cpp_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	wasmer "github.com/wasmerio/wasmer-go/wasmer"

	db "sigmaos/debug"
	//	"sigmaos/apps/cossim"
	//	cossimproto "sigmaos/apps/cossim/proto"
	//	cossimsrv "sigmaos/apps/cossim/srv"
	//	"sigmaos/apps/epcache"
	//	epcachesrv "sigmaos/apps/epcache/srv"
	//	spinproto "sigmaos/apps/spin/proto"
	//	echoproto "sigmaos/example/example_echo_server/proto"
	//	"sigmaos/proc"
	//	rpcncclnt "sigmaos/rpc/clnt/netconn"
	//	sp "sigmaos/sigmap"
	//	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

func TestHelloWorld(t *testing.T) {
	wasmScript, err := os.ReadFile("/home/arielck/sigmaos/wasm/helloworld.wasm")
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

	// Gets the `sum` exported function from the WebAssembly instance.
	sum, err := instance.Exports.GetFunction("sum")
	if !assert.Nil(t, err, "Err get wasm function: %v", err) {
		return
	}

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := sum(5, 37)
	if !assert.Nil(t, err, "Err call wasm function: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Result: %v", result) // 42!
}

func TestLatency(t *testing.T) {
	const (
		NTRIAL = 10
	)
	wasmScript, err := os.ReadFile("/home/arielck/sigmaos/wasm/helloworld.wasm")
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

	var sum wasmer.NativeFunction

	durs = make([]time.Duration, 0, NTRIAL)
	start = time.Now()
	for i := 0; i < NTRIAL; i++ {
		start := time.Now()
		// Gets the `sum` exported function from the WebAssembly instance.
		sum, err = instance.Exports.GetFunction("sum")
		if !assert.Nil(t, err, "Err get wasm function: %v", err) {
			return
		}
		durs = append(durs, time.Since(start))
	}
	db.DPrintf(db.TEST, "Wasm function retrieval latency:\n\tAvg: %v\n\tEach:%v", time.Since(start)/NTRIAL, durs)

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := sum(5, 37)
	if !assert.Nil(t, err, "Err call wasm function: %v", err) {
		return
	}

	assert.Equal(t, result.(int32), int32(42))
}
