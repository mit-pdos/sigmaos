package main

import (
	"os"
	"plugin"
	"syscall"
	"time"

	db "sigmaos/debug"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v PLUGIN_PATH FUNC_NAME", os.Args[0])
	}
	ru := &syscall.Rusage{}
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, ru); err != nil {
		db.DFatalf("Err getrusage: %v", err)
	}
	db.DPrintf(db.ALWAYS, "start minpf:%d majpf:%d", ru.Minflt, ru.Majflt)

	pluginPath := os.Args[1]
	funcName := os.Args[2]

	if err := syscall.Getrusage(syscall.RUSAGE_SELF, ru); err != nil {
		db.DFatalf("Err getrusage: %v", err)
	}
	db.DPrintf(db.ALWAYS, "pre open minpf:%d majpf:%d", ru.Minflt, ru.Majflt)

	start := time.Now()
	p, err := plugin.Open(pluginPath)
	if err != nil {
		db.DFatalf("Err open plugin: %v", err)
	}
	db.DPrintf(db.ALWAYS, "Time plugin.Open: %v", time.Since(start))

	if err := syscall.Getrusage(syscall.RUSAGE_SELF, ru); err != nil {
		db.DFatalf("Err getrusage: %v", err)
	}
	db.DPrintf(db.ALWAYS, "opened minpf:%d majpf:%d", ru.Minflt, ru.Majflt)

	start = time.Now()
	fn, err := p.Lookup(funcName)
	if err != nil {
		panic(err)
	}
	db.DPrintf(db.ALWAYS, "Time plugin.Lookup: %v", time.Since(start))

	if err := syscall.Getrusage(syscall.RUSAGE_SELF, ru); err != nil {
		db.DFatalf("Err getrusage: %v", err)
	}
	db.DPrintf(db.ALWAYS, "looked up minpf:%d majpf:%d", ru.Minflt, ru.Majflt)

	start = time.Now()
	fn.(func())()
	db.DPrintf(db.ALWAYS, "Time plugin.Lookup: %v", time.Since(start))

	if err := syscall.Getrusage(syscall.RUSAGE_SELF, ru); err != nil {
		db.DFatalf("Err getrusage: %v", err)
	}
	db.DPrintf(db.ALWAYS, "done minpf:%d majpf:%d", ru.Minflt, ru.Majflt)
}
