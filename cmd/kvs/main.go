package main

import (
	memorystore "kv_store/internal/memory_store"
	"kv_store/internal/spec"
	"log"
	"net/http"
)

func main() {

	http.HandleFunc(spec.KeyStorePath, memorystore.MemoryStoreHandler)

	log.Print(http.ListenAndServe(":8000", nil))

}
