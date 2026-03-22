package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	memorystore "kv_store/internal/memory_store"
	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile = flag.String("memprofile", "", "write memory profile to `file`")
)

func main() {
	flag.Parse()
	closer := utility.ProfileCPU(cpuprofile)
	if closer != nil {
		defer closer()
	}

	go func() {
		http.HandleFunc(spec.KeyStorePath, memorystore.MemoryStoreHandler)
		log.Println("Starting server on :8000")
		if err := http.ListenAndServe(":8000", nil); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down...")

	// ensure all operation necessary items are written to disk
	memorystore.CloseMemoryStore()
	utility.ProfileMemory(memprofile)
}
