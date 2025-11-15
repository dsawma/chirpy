package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/dsawma/chirpy/internal/database"
)

/* holds atomic Int32 counter shared across handlers safely */
type apiConfig struct {
	fileserverHits atomic.Int32
	db *database.Queries
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
	</html>`, hits)
}

/* sets counter to 0 with Store(0) returns OK(200) */
func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func handler (w http.ResponseWriter, r *http.Request) {
	type requestStruct struct {
		Body string `json:"body"`
	}

	type validateResp struct {
		CleanedBody string `json:"cleaned_body"`
	}
	decoder := json.NewDecoder(r.Body)
	req := requestStruct{}
	err := decoder.Decode(&req)
	if err != nil  {
		log.Printf("error %s", err)
		w.WriteHeader(500) 
		return 
	}
	if len(req.Body) > 140{
		respondWithError(w,400, "Chirp is too long")

	}else {
		wordsFind := map[string]bool{"kerfuffle":true, "sharbert":true, "fornax":true}
		wordsSplit := strings.Split(req.Body, " ")
		for i, word := range wordsSplit {
			if wordsFind[strings.ToLower(word)] {
				wordsSplit[i] = "****"
			}
		}
		cleaned := strings.Join(wordsSplit, " ")

		respondWithJSON(w , 200,  validateResp{CleanedBody: cleaned})
	}
	
}

func respondWithError(w http.ResponseWriter, code int, msg string){
	type errorResp struct {
  		Error string `json:"error"`
	}
	er := errorResp{ Error: msg}
	dat,er2:= json.Marshal(er)
	if er2 != nil  {
		log.Printf("error %s", er2)
		w.WriteHeader(500) 
		return 
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat,er2 := json.Marshal(payload)
	if er2 != nil  {
		log.Printf("error %s", er2)
		w.WriteHeader(500) 
		return 
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func main() {
	godotenv.Load() //loads env varaibles from .env file 
	dbURL := os.Getenv("DB_URL")  //reads db connection string from env
	if dbURL == "" {
    	log.Fatal("DB_URL is missing")
	}	
	db, err := sql.Open("postgres", dbURL) // open a db handle using the postgres driver(imported for side effects with _)
	if err != nil  {
		log.Fatal(err)
	}
	defer db.Close() 
	if err := db.Ping(); err != nil { //create type-safe query helper and attached to apiconfig
    	log.Fatal(err) 					//allows us to run queries via apiCfg.db
	}

	/* creates shared config*/
	apiCfg := &apiConfig{ db: database.New(db)}
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
	mux.HandleFunc("/api/healthz", handlerReadiness)
	mux.HandleFunc("/admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("/api/validate_chirp", handler)

	/* resets counter only on POST */
	mux.HandleFunc("/admin/reset", apiCfg.handlerReset)

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
