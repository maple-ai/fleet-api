package syrup

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type H map[string]interface{}

func New(r *mux.Router, middleware ...http.HandlerFunc) Router {
	return Router{r}
}

func Bind(w http.ResponseWriter, r *http.Request, out interface{}) error {
	err := json.NewDecoder(r.Body).Decode(out)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	return err
}

func WriteJSON(w http.ResponseWriter, code int, out interface{}) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(out); err != nil {
		panic(err)
	}
}

func WriteJSONChecksum(w http.ResponseWriter, code int, out interface{}) {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(out); err != nil {
		panic(err)
	}

	// calculate checksum and set as header
	sum := sha256.Sum256(b.Bytes())
	w.Header().Set("x-checksum", fmt.Sprintf("%x", sum))
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)

	b.WriteTo(w)
}
