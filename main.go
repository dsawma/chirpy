package main

import (
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(handler http.Handler) http.Handler {
	cfg.fileserverHits.Add(1)
}
func (cfg *apiConfig) numOfRequest(handler http.Handler) http.Handler {
	cfg.fileserverHits.Add(1)
}

func NewConfig(i int32) *apiConfig {
	c := &apiConfig{}
	c.fileserverHits.Store(i)
	return c
}

func main() {
	apiC := NewConfig(0)
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", apiC.middlewareMetricsInc(http.StripPrefix("/app", fileServer)))
	mux.HandleFunc("/healthz", handlerReadiness)

	hits := http.NewServeMux()
	hits.Handle("")
	server := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	server.ListenAndServe()
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}


