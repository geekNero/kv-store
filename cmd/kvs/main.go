package main

import (
	"context"
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

	err := memorystore.SetupMemoryStore()
	if err != nil {
		log.Fatal("failed to setup memory store")
	}

	mux := http.NewServeMux()
	mux.HandleFunc(spec.KeyStorePath, memorystore.MemoryStoreHandler)

	server := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	go func() {
		log.Println("Starting server on :8000")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	server.Shutdown(context.Background())
	// ensure all operation necessary items are written to disk
	memorystore.CloseMemoryStore()
	utility.ProfileMemory(memprofile)
}
