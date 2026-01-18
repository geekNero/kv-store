package memorystore

import (
	"encoding/json"
	"fmt"
	"kv_store/internal/spec"
	"kv_store/internal/utility"
	"net/http"
	"strings"
)

func MemoryStoreHandler(w http.ResponseWriter, r *http.Request) {

	key := strings.Split(r.URL.Path, spec.KeyStorePath)[1]

	if !utility.IsASCII(key) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		val, found := handleGet(key)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(val))
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
		handlePut(&body)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
