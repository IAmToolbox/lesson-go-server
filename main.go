package main

import _ "github.com/lib/pq"

import (
	"fmt"
	"strings"
	"slices"
	"errors"
	"net/http"
	"os"
	"log"
	"time"
	"sync/atomic"
	"encoding/json"
	"database/sql"
	"github.com/iamtoolbox/lesson-go-server/internal/database"
	"github.com/joho/godotenv"
	"github.com/google/uuid"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
	platform string
}

type User struct {
	ID uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email string `json:"email"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(403)
		return
	}

	cfg.fileserverHits.Swap(0)
	err := cfg.dbQueries.DeleteAllUsers(r.Context())
	if err != nil {
		log.Fatalf("couldn't delete all users: %w", err)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("Server Reset"))
	if err != nil {
		log.Fatalf("failed to write response: %w", err)
	}
}

func (cfg *apiConfig) createChirp(w http.ResponseWriter, r *http.Request) { // This function will decode and encode JSON
	type chirp struct {
		Body string `json:"body"`
		UserID uuid.NullUUID `json:"user_id"`
	}
	type resErr struct {
		Error string `json:"error"`
	}
	type resVal struct {
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body string `json:"body"`
		UserID uuid.NullUUID `json:"user_id"`
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
			return
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
			return
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
	chirpData.Body = cleanWords

	chirpArgs := database.CreateChirpParams{
		Body: chirpData.Body,
		UserID: chirpData.UserID,
	}
	newChirp, err := cfg.dbQueries.CreateChirp(r.Context(), chirpArgs)
	if err != nil {
		log.Fatalf("Couldn't add new chirp: %w", err)
	}

	resBody := resVal{
		ID: newChirp.ID,
		CreatedAt: newChirp.CreatedAt,
		UpdatedAt: newChirp.UpdatedAt,
		Body: newChirp.Body,
		UserID: newChirp.UserID,
	}
	data, err := json.Marshal(resBody)
	if err != nil {
		log.Fatalf("Couldn't marshal response: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(data)
}

func (cfg *apiConfig) getAllChirps(w http.ResponseWriter, r *http.Request) {
	type resVal struct {
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body string `json:"body"`
		UserID uuid.NullUUID `json:"user_id"`
	}

	chirpsAsc, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		log.Fatalf("couldn't get chirps: %w", err)
	}
	resBody := []resVal{}
	for _, chirp := range chirpsAsc {
		resBody = append(resBody, resVal{
			ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID,
		})
	}
	data, err := json.Marshal(resBody)
	if err != nil {
		log.Fatalf("couldn't marshal response: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func (cfg *apiConfig) getChirpByID(w http.ResponseWriter, r *http.Request) {
	type resVal struct {
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body string `json:"body"`
		UserID uuid.NullUUID `json:"user_id"`
	}

	parsedUUID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		log.Fatalf("couldn't parse uuid: %w", err)
	}

	chirp, err := cfg.dbQueries.GetChirpByID(r.Context(), parsedUUID)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(404)
		return
	}

	resBody := resVal{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
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
	// Database init
	godotenv.Load() // Load environment variables
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Couldn't open SQL database: %w", err)
	}

	// Server code
	mux := http.NewServeMux()
	config := apiConfig{
		dbQueries: database.New(db),
		platform: os.Getenv("PLATFORM"),
	}
	mux.Handle("/app/", config.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir("."))))) // Look at all those closing parenthesis brooo
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			log.Fatalf("failed to write response: %w", err)
		}
	})
	mux.HandleFunc("POST /api/chirps", config.createChirp)
	mux.HandleFunc("GET /api/chirps", config.getAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", config.getChirpByID)
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, req *http.Request) {
		type reqData struct {
			Email string `json:"email"`
		}
		decoder := json.NewDecoder(req.Body)
		reqDecoded := reqData{}
		err := decoder.Decode(&reqDecoded)
		if err != nil {
			log.Fatalf("couldn't decode json data: %w", err)
			return
		}
		newUser, err := config.dbQueries.CreateUser(req.Context(), reqDecoded.Email)
		if err != nil {
			log.Fatalf("couldn't create user: %w", err)
			return
		}

		resBody := User{
			ID: newUser.ID,
			CreatedAt: newUser.CreatedAt,
			UpdatedAt: newUser.UpdatedAt,
			Email: newUser.Email,
		}
		data, err := json.Marshal(resBody)
		if err != nil {
			log.Fatalf("Couldn't marshal response: %w", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, err = w.Write(data)
		if err != nil {
			log.Fatalf("couldn't write response: %w", err)
		}
	})

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

	err = server.ListenAndServe()
	if err != nil {
		fmt.Println("Server has been closed")
		os.Exit(0)
	}
}
