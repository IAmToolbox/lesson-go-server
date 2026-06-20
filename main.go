package main

import (
	"fmt"
	"net/http"
	"os"
	"log"
	"sync/atomic"
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

func main() {
	mux := http.NewServeMux()
	var config apiConfig
	mux.Handle("/app/", config.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir("."))))) // Look at all those closing parenthesis brooo
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			log.Fatalf("failed to write response: %w", err)
		}
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(fmt.Sprintf("Hits: %v", config.fileserverHits.Load())))
		if err != nil {
			log.Fatalf("failed to write response: %w", err)
		}
	})
	mux.HandleFunc("POST /reset", config.metricsReset)
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
