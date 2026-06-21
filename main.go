package main

import (
	"fmt"
	"strings"
	"slices"
	"net/http"
	"os"
	"log"
	"sync/atomic"
	"encoding/json"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Swap(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Hits Reset"))
	if err != nil {
		log.Fatalf("failed to write response: %w", err)
	}
}

func validateChirp(w http.ResponseWriter, r *http.Request) { // This function will decode and encode JSON
	type chirp struct {
		Body string `json:"body"`
	}
	type resErr struct {
		Error string `json:"error"`
	}
	type resVal struct {
		CleanedBody string `json:"cleaned_body"`
	}

	decoder:= json.NewDecoder(r.Body)
	chirpData := chirp{}
	err := decoder.Decode(&chirpData)
	if err != nil {
		resBody := resErr{
			Error: "Couldn't decode request",
		}
		data, err := json.Marshal(resBody)
		if err != nil {
			log.Fatalf("Couldn't marshal response: %w", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(data)
		return
	}

	if len(chirpData.Body) > 140 {
		resBody := resErr{
			Error: "Chirp is too long",
		}
		data, err := json.Marshal(resBody)
		if err != nil {
			log.Fatalf("Couldn't marshal response: %w", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(data)
		return
	}
	bodyWords := strings.Split(chirpData.Body, " ")
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	for i, word := range(bodyWords) {
		if slices.Contains(badWords, strings.ToLower(word)) {
			bodyWords[i] = "****"
		}
	}
	cleanWords := strings.Join(bodyWords, " ")

	resBody := resVal{
		CleanedBody: cleanWords,
	}
	data, err := json.Marshal(resBody)
	if err != nil {
		log.Fatalf("Couldn't marshal response: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func main() {
	mux := http.NewServeMux()
	var config apiConfig
	mux.Handle("/app/", config.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir("."))))) // Look at all those closing parenthesis brooo
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			log.Fatalf("failed to write response: %w", err)
		}
	})
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(fmt.Sprintf(`<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>`, config.fileserverHits.Load())))
		if err != nil {
			log.Fatalf("failed to write response: %w", err)
		}
	})
	mux.HandleFunc("POST /admin/reset", config.metricsReset)
	server := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Server has been closed")
		os.Exit(0)
	}
}
