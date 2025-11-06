package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

/* holds atomic Int32 counter shared across handlers safely */
type apiConfig struct {
	fileserverHits atomic.Int32
}

/*
	 middleware to count request hitting the file server
		 wrap http.Handler and returns new one
		logs each request path, increments fileserverHits,
		calls wrapped handler
*/
func (cfg *apiConfig) middlewareMetricsInc(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("metrics middleware hit: %s", r.URL.Path)
		cfg.fileserverHits.Add(1)
		handler.ServeHTTP(w, r)
	})
}

/*
reads current counter with Load()
sets content-type to text/plain
respond with "Hits:N"
*/
func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	hits := cfg.fileserverHits.Load()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "Hits: %d", hits)
}

/* sets counter to 0 with Store(0) returns OK(200) */
func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func main() {

	/* creates shared config*/
	apiCfg := &apiConfig{}
	/* cretes ServeMux router */
	mux := http.NewServeMux()

	/* serves static files from current directory */
	fileServer := http.FileServer(http.Dir("."))
	/* routs all /app/* paths to file server after stripping /app/ from URL
	wraps with metric middleware to increment each time
	*/
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", fileServer)))

	/* opens /healthz(shows endpoint "OK")
	and /metrics(shows hit count) with only GET
	*/
	mux.HandleFunc("GET /healthz", handlerReadiness)
	mux.HandleFunc("GET /metrics", apiCfg.handlerMetrics)

	/* resets counter only on POST */
	mux.HandleFunc("POST /reset", apiCfg.handlerReset)

	/* starst localhost:8080 with mux as handler */
	server := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	server.ListenAndServe()
}

/* sets plain text Content-type, sends 200(OK) */
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}
