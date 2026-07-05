package main

import (
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// import "github.com/google/uuid"

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"database/sql"

	_ "github.com/lib/pq"
	"github.com/zeelna/chirpy/internal/database"
)

const (
	project_root_path = "/"
	current_directory = "."
	port              = "8080"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
}

// Heplful Go doc links:
// // https://pkg.go.dev/net/http#ServeMux.Handle
// type Handler: https://pkg.go.dev/net/http#Handler

// https://pkg.go.dev/net/http#ResponseWriter
//https://pkg.go.dev/net/http#ResponseWriter.Write

func main() {
	// #1 cmd: go get github.com/joho/godotenv
	// instead, I added into go.mod

	// #1 load the .env into your environment to access the 'db connection string'
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error opening database: %s", err)
	}
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %s", err)
	}
	// #2 use SQLC generated 'database' package to create a new <*database.Queries> and store into apiConfig struct
	// so that handlers can access it
	dbQueries := database.New(db)

	// therefore, we add resulting 'dbQueries' into our db field
	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
	}

	// --------------------------------------------------------
	// We have many handlers, we don't want potential conflicts with the fileserver handler.
	// Updated the fileserver to use the /app/ path instead of /.
	// Not only will you need to mux.Handle the /app/ path,
	// you'll also need to strip the /app prefix from the request path before passing it to the fileserver handler.
	// You can do this using the http.StripPrefix function.
	serverMux := http.NewServeMux()
	// GET /app redirects to /app/ (to avoid GET /app vs GET /app/
	serverMux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
	})
	// GET /app/ serves files
	// Motivation: now we remove /app from /app/, resulting in '/'. This is required. We cannot simply use serverMux.handle("/", ...)
	serverMux.Handle("/app/", apiCfg.middlewareMetricsInc(
		http.StripPrefix("/app", http.FileServer(http.Dir(current_directory))),
	),
	)
	// --------------------------------------------------------
	// Now that we've added a new handler (for path '/healthz' , we don't want potential conflicts with the fileserver handler.)
	// Updated the fileserver to use the /app/ path instead of /. And for that, we used http.StripPrefix inside of serverMux.Handle() function call
	// --------------------------------------------------------

	// GET /healthz -- a create 'readiness endpoint' for Chirpy server.
	// Motivation: Readiness endpoints are commonly used by external systems to check if our server is ready to receive traffic.
	serverMux.HandleFunc("GET /api/healthz", apiCfg.handlerHealth) // Later this endpoint can be enhanced to return a 503 Service Unavailable status code if the server is not ready.
	// --------------------------------------------------------
	// GET /metrics -- how many people are viewing the site (until server is turned off)
	// motivation: // how many requests are being made to serve our homepage - in essence, they want to know
	serverMux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics) // return the count as plain text in the response body.
	// --------------------------------------------------------

	// GET /metrics -- reset to '0' many people are viewing the site!
	serverMux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	// --------------------------------------------------------

	// GET /metrics -- reset to '0' many people are viewing the site!
	serverMux.HandleFunc("POST /api/validate_chirp", apiCfg.handlerValidateChirp)

	// GET /metrics -- reset to '0' many people are viewing the site!
	serverMux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)
	// --------------------------------------------------------

	server := http.Server{
		Handler: serverMux,
		Addr:    ":" + port,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
	return
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	// The idea is that instead of running the increment immediately,
	// you delay it by wrapping it in a function that runs later (when a request actually arrives).

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//fmt.Println("request received!") // runs on each request
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r) //
	})
	// do not forge the pattern here - middleware wraps an anonymous-function with different function signature
}

func (cfg *apiConfig) handlerHealth(w http.ResponseWriter, req *http.Request) {
	// Header:
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Status Code: Send HTTP 200/ok
	w.WriteHeader(http.StatusOK)

	// Send 'OK' in response body
	if _, err := w.Write([]byte("OK\n")); err != nil {
		log.Fatal(err)
	}
	// The endpoint should simply return a 200 OK status code indicating that it has started up successfully and is listening for traffic
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, req *http.Request) {
	// Header:
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Status Code: Send HTTP 200/ok
	w.WriteHeader(http.StatusOK)

	// Respone body
	hits := cfg.fileserverHits.Load()
	body := fmt.Sprintf(
		`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, hits)

	// Send count 'cfg.fileserverHits.Load()' in response body. Be sure to convert into []byte slice.
	if _, err := w.Write([]byte(body)); err != nil {
		log.Fatal(err)
	}
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, req *http.Request) {
	// reset the count of visits to the server
	cfg.fileserverHits.Store(0)

	// Header:
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Status Code: Send HTTP 200/ok
	w.WriteHeader(http.StatusOK)

	// Body: Send 'OK'
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Fatal(err)
	}
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, req *http.Request) {
	type reqParameters struct {
		Email string `json:"email"`
	}
	// Decode JSON Request Body
	decoder := json.NewDecoder(req.Body)
	reqParams := reqParameters{}
	errorEncoding := decoder.Decode(&reqParams)
	// -- bad path --
	if errorEncoding != nil {
		log.Printf("Error decoding parameters: %s", errorEncoding)
		_respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	// Every http.Request has a context. With this, database work is tied to the lifetime of the HTTP Request.
	ctx := req.Context() // You'll also see context.Background() in Go code.
	// For web handlers, prefer r.Context(). It carries the cancellation signal for the specific request you're handling.
	// It's useful when a Context is expected but there's no incoming request or parent operation to start from – like in startup code or a background job.

	// Your SQLC method expects a context.Context as its first argument. In an HTTP handler, use the request context from r.Context().
	user, err := cfg.db.CreateUser(ctx, reqParams.Email)
	if err != nil {
		_respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	// happy path
	type respParameters struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}
	respParams := respParameters{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	_respondWithJSON(w, http.StatusCreated, respParams)
	return
}

func (cfg *apiConfig) handlerValidateChirp(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	type reqParameters struct {
		Body string `json:"body"`
	}
	// Decode JSON Request Body
	decoder := json.NewDecoder(req.Body)
	reqParams := reqParameters{}
	errorEncoding := decoder.Decode(&reqParams)
	// -- bad path --
	if errorEncoding != nil {
		log.Printf("Error decoding parameters: %s", errorEncoding)
		_respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	// -- end of bad path --

	// -- start of happy path --
	// Validate Chirp length is less than or equal to 140 characters.
	if len(reqParams.Body) > 140 {
		_respondWithError(w, http.StatusBadRequest, "Chirp is too long")
	} else if len(reqParams.Body) <= 0 {
		_respondWithError(w, http.StatusBadRequest, "Chirp cannot be empty")
	} else {
		type respParams struct {
			CleanedBody string `json:"cleaned_body"`
		}
		cleaned := replaceProfaneWords(reqParams.Body)
		_respondWithJSON(w, http.StatusOK, respParams{CleanedBody: cleaned})
	}
	return
	// -- end of happy path --
}

// you can compile a binary and run server (in the background):
// go build -o out && ./out
// note: Ctrl + C terminates the server.

// 1. curl http://localhost:8080/
// 2. curl http://localhost:8080/assets/logo.png
//  curl -X POST "http://localhost:8080/api/validate_chirp" -H "Content-Type: application/json" -d '{"chirp":"hello"}'

func replaceProfaneWords(msg string) string {
	// words to replace
	profanities := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	} // map[string]struct{} is an efficient set: keys are the words you want to replace, and the value is an empty struct{} since you don’t need any extra data.
	// An alternative would be a []string and looping to find a match, but that’s O(n) per word instead of constant-time

	words := strings.Fields(msg) // splits on whitespace; punctuation stays attached
	for i := range words {
		// lowercase for matching; punctuation will prevent an exact match (e.g., "Sharbert!" won't match "sharbert")
		w := strings.ToLower(words[i])
		if _, ok := profanities[w]; ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

func _respondWithError(w http.ResponseWriter, statusCode int, msg string) {
	// Create JSON Response body type
	type ErrorVals struct {
		Error string `json:"error"`
	}
	_respondWithJSON(w, statusCode, ErrorVals{Error: msg})
}

func _respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// Encode JSON Response body
	data, errorMarshalling := json.Marshal(payload)

	// Sad path.
	if errorMarshalling != nil {
		log.Printf("Error marshalling HTTP Response JSON: %s", errorMarshalling)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	// Happy path.
	w.WriteHeader(statusCode)
	if _, err := w.Write(data); err != nil {
		log.Printf("Error writing HTTP response: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	return
}
