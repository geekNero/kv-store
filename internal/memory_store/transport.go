package memorystore

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"kv_store/internal/spec"
	"kv_store/internal/utility"
)

func MemoryStoreHandler(w http.ResponseWriter, r *http.Request) {
	key := strings.Split(r.URL.Path, spec.KeyStorePath)[1]

	if !utility.IsASCII(key) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		val := handleGet(key)
		if val == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, err := w.Write([]byte(*val))
		if err != nil {
			log.Println("failed to write response body, error: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}

	case http.MethodPut:

		var body spec.PutRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&body)
		if err != nil {
			fmt.Printf("failed to decode req body: %s", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		body.Key = key
		accepted := handlePut(&body)
		if accepted {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

	case http.MethodDelete:
		ok, err := handleDelete(key)
		if ok {
			w.WriteHeader(http.StatusOK)
		} else if err == nil {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func SetupMemoryStore() error {
	err := loadWAL()
	if err != nil {
		log.Println("failed to loadWAL file, error: ", err.Error())
		return err
	}
	return nil
}

func CloseMemoryStore() {
	err := flushManifest()
	if err != nil {
		log.Println("failed to save manifest, error: ", err.Error())
		err = os.Remove(utility.ManifestName)
		if err != nil {
			log.Println("failed to delete existing manifest, error: ", err.Error())
		}
	}

	closeWAL()
}
