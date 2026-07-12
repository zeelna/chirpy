package main

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/zeelna/chirpy/internal/auth"
)

// import "github.com/google/uuid"

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"database/sql"

	_ "github.com/lib/pq"
	"github.com/zeelna/chirpy/internal/database"
)

const (
	project_root_path = "/"
	current_directory = "."
	port              = "8080"
	platformDev       = "dev"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwtSecretKey   string
}

// Heplful Go doc links:
// https://pkg.go.dev/net/http#ServeMux.Handle
// https://pkg.go.dev/net/http#Handler
// https://pkg.go.dev/net/http#ResponseWriter
//https://pkg.go.dev/net/http#ResponseWriter.Write

// 1. curl http://localhost:8080/
// 2. curl http://localhost:8080/assets/logo.png
//  curl -X POST "http://localhost:8080/api/validate_chirp" -H "Content-Type: application/json" -d '{"chirp":"hello"}'

type LoginResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	JwtToken  string    `json:"token"`
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type ChirpResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

// you can compile a binary and run server (in the background):
// go build -o out && ./out
// note: Ctrl + C terminates the server.
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

	// Load the JWT secre
	jwtSecretKey := os.Getenv("JWT_SIGNING_KEY")

	// #2 use SQLC generated 'database' package to create a new <*database.Queries> and store into apiConfig struct
	// so that handlers can access it
	dbQueries := database.New(db)

	// therefore, we add resulting 'dbQueries' into our db field
	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       os.Getenv("PLATFORM"),
		jwtSecretKey:   jwtSecretKey,
	}
	// os.Getenv("PLATFORM") -> reading value of key 'PLATFORM' from .env into apiConfig struct

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
	// POST /api/users  -- add a new users with HTTP Request Body {'email': 'abc@xyz.com'}
	serverMux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)

	// GET /api/users with JSON-body {'email': 'xyz'}
	//-- retrieve ID of user via HTTP Request Body {'email': 'abc@xyz.com'}
	serverMux.HandleFunc("GET /api/users/", apiCfg.handlerGetUserByEmail)

	// PUT /api/users - only users can update their own (but not others') email and password. It requires
	// 1) access token in the HTTP Request Header
	// 2) a new 'password' and a new 'email' in the request body
	serverMux.HandleFunc("PUT /api/users", apiCfg.handlerUpdateCredentials)

	// POST /api/chirps
	// ported logic into 'handlerCreateChrip' and delete duplicate this validate handler
	//serverMux.HandleFunc("POST /api/validate_chirp", apiCfg.handlerValidateChirp)
	serverMux.HandleFunc("POST /api/chirps", apiCfg.handlerCreateChirp)

	// GET /api/chirps
	serverMux.HandleFunc("GET /api/chirps", apiCfg.handlerGetAllChirps)

	// GET /api/chirp with UUID
	serverMux.HandleFunc("GET /api/chirps/{id}", apiCfg.handlerGetChirp)

	// POST /api/login with UUID
	serverMux.HandleFunc("POST /api/login", apiCfg.handlerLogin)

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
	// Headers:
	w.Header().Set("Content-Type", "plain/text; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// ensures that this extremely dangerous endpoint can be accessed only in a local development environment.
	if cfg.platform != platformDev {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// reset the count of visits to the server (render to HTML)
	cfg.fileserverHits.Store(0)

	// Reset the entire 'users' table to empty (keeping schema)
	if err := cfg.db.DeleteAllUsers(req.Context()); err != nil {
		log.Printf("Error deleting all users: %v", err)
		if _, err := w.Write([]byte("Failed to delete")); err != nil {
			log.Fatal(err)
		}
		//_respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	// -- happy path --
	// Status Code: Send HTTP 200/ok
	w.WriteHeader(http.StatusOK)

	// Body: Send 'OK'
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Fatal(err)
	}
}

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, req *http.Request) {
	type reqParameters struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
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

	// ----convert {'expires_in_seconds': 1} to time.Duration() ----
	// If it's specified by the client, use it as the expiration time. If it's not specified, use a default expiration time of 1 hour. If the client specified a number over 1 hour, use 1 hour as the expiration time.
	if reqParams.ExpiresInSeconds == 0 || reqParams.ExpiresInSeconds > 3600 { // 0 seconds
		reqParams.ExpiresInSeconds = 3600
	}
	// time.Duration() by default is using 'nanoseconds'. Here is conversion
	expiresIn := time.Duration(reqParams.ExpiresInSeconds) * time.Second
	// --------------------------------------------------------------

	// -- Database operation -> SELECT * FROM users WHERE email = ...;
	// Searching a 'user' entry in 'users' table via HTTP Request body {'email': '<any_value>'}
	user, err := cfg.db.GetUserByEmail(req.Context(), reqParams.Email)
	if err != nil {
		_respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	isCorrectPassword, err := auth.CheckPasswordHash(reqParams.Password, user.HashedPassword)
	if !isCorrectPassword || err != nil {
		_respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	jwtToken, err := auth.MakeJWT(user.ID, cfg.jwtSecretKey, expiresIn)
	if err != nil {
		_respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("%v", err))
		return
	}

	// -- happy path -- Once successfully received from db, write into JSON for HTTP Response Body.
	_respondWithJSON(w, http.StatusOK, LoginResponse{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		JwtToken:  jwtToken,
	})
	return
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, req *http.Request) {
	type reqParameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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

	// Convert plain-text password into Hash
	// todo: next assignment will remove this UNSAFE implementation, will use JWT
	hashedPassword, err := auth.HashPassword(reqParams.Password)
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
		return
	}

	// Your SQLC method expects a context.Context as its first argument. In an HTTP handler, use the request context from r.Context().
	user, err := cfg.db.CreateUser(ctx, database.CreateUserParams{
		Email:          reqParams.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, "User already exists")
		return
	}

	// HTTP Response JSON body (type 'UserResponse' struct defined above)
	respParams := UserResponse{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	_respondWithJSON(w, http.StatusCreated, respParams)
	return
}

func (cfg *apiConfig) handlerGetUserByEmail(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Type define the struct
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

	// -- Database operation -> SELECT * FROM users WHERE email = ...;
	// Searching a 'user' entry in 'users' table via HTTP Request body {'email': '<any_value>'}
	user, err := cfg.db.GetUserByEmail(req.Context(), reqParams.Email)
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, "User does not exist")
		return
	}
	// -- happy path -- Once successfully received from db, write into JSON for HTTP Response Body.
	// Type define the struct, and create inline instance with _respondWithJSON()
	type respParams struct {
		ID uuid.UUID `json:"user_id"`
	}
	_respondWithJSON(w, http.StatusOK, respParams{ID: user.ID})
	return

}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, req *http.Request) {
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
	// ----- Authorization ---------------------------------
	// Authorization: To creat a new 'Chirp' you must be logged-in, that is:
	// + Authorization: Bearer <token>
	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		_respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("%v", err))
		return
	}
	//Verify JWT string is correct
	tokenSecret := cfg.jwtSecretKey
	uuidFromJWT, err := auth.ValidateJWT(tokenString, tokenSecret)
	if err != nil {
		_respondWithError(w, http.StatusUnauthorized, "Authorization unsuccessful. Invalid JWT or secret")
		return
	}
	// ---end of Authorization --------------------------------------
	user, err := cfg.db.GetUser(req.Context(), uuidFromJWT)
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, "User does not exist")
		return
	}

	// Validate Chirp length is less than or equal to 140 characters.
	if len(reqParams.Body) > 140 {
		_respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	} else if len(reqParams.Body) <= 0 {
		_respondWithError(w, http.StatusBadRequest, "Chirp cannot be empty")
		return
	}
	// -- end of bad path --

	// -- start of happy path --
	// Work with response body parameters
	cleanedBody := replaceProfaneWords(reqParams.Body)
	//strUserID := fmt.Sprintf("${%v}", reqParams.UserID)

	// Update 'chirps' database with new chrip
	chirp, err := cfg.db.CreateChirp(req.Context(), database.CreateChirpParams{
		Body:   cleanedBody,
		UserID: user.ID,
	})
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	resp := ChirpResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
	_respondWithJSON(w, http.StatusCreated, resp)
	return
	// -- end of happy path --
}

func (cfg *apiConfig) handlerGetAllChirps(w http.ResponseWriter, req *http.Request) {
	// Headers
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Database operation to extract the all 'chirp' entries from table 'chirps'
	chirps, err := cfg.db.GetAllChirps(req.Context())
	// sad path
	if err != nil {
		_respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	// -- happy path --
	// HTTP Response JSON Body must be snake-cased {'updated_at': ...}
	// therefore, we cannot simply use []Chirp from SQLC generated file 'chirps.sql.go'
	// if that wouldn't be the case, we could call fn; _respondWithJSON(w, http.StatusOK, chrips)
	responses := []ChirpResponse{}
	for _, chirp := range chirps {
		responses = append(responses, ChirpResponse{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}
	_respondWithJSON(w, http.StatusOK, responses)
	return
}

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, req *http.Request) {
	// Headers
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// GET /api/chirps/{id} into uuid.UUID type
	idString := req.PathValue("id")
	uuidValue, err := uuid.Parse(idString)
	//  -- HTTP Request PathVariable "id" failed to convert into Google's UUID failed --
	if err != nil {
		_respondWithError(w, http.StatusNotFound, "Chirp does not exist.")
		return
	}

	// Database operation to extract the 'chrip' entry from table 'chirps'
	chirp, err := cfg.db.GetChirp(req.Context(), uuidValue)
	if err != nil {
		_respondWithError(w, http.StatusNotFound, "Chirp does not exist.")
		return
	}
	// Happy path
	_respondWithJSON(w, http.StatusOK, ChirpResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
	return
	// -- end of happy path --
}
func (cfg *apiConfig) handlerUpdateCredentials(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	type reqParameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
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

	// ----- Authorization - before any operation, Verify that: 'Logged-in' by JWT Access Token in HTTP Header--------
	// Authorization: To update credentials, you must be logged-in, that is:
	// The HTTP Header will look like this:  ´Authorization: Bearer <token>´
	tokenString, err := auth.GetBearerToken(req.Header)
	if err != nil {
		_respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("%v", err))
		return
	}
	// Verify JWT string is correct
	tokenSecret := cfg.jwtSecretKey
	uuidFromJWT, err := auth.ValidateJWT(tokenString, tokenSecret)
	if err != nil {
		_respondWithError(w, http.StatusUnauthorized, "Authorization unsuccessful. Invalid JWT or secret")
		return
	}
	// JWT Token returns the UUID of specific user. NOW confirmed 'user' exists in 'users' table in database
	user, err := cfg.db.GetUser(req.Context(), uuidFromJWT)
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, "User does not exist")
		return
	}
	// -- end of JWT Validation --------------------------------------
	// -- Authorization complete ---------------------------

	// --  Authorized User acquired from JWT, continue to update the DB credentials --
	// 0) update 'updated_at' entry, 1) hash new password, 2) overwrite with new hashed-password, 3) overwrite with new email in db,
	newEmail := reqParams.Email
	newHashedPassword, err := auth.HashPassword(reqParams.Password)
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
		return
	}

	// In directory <project_root>/sql/queries, look at the `-- name: UpdateUser :one`
	// 0) updated_at is changed with NOW() because we are modifying existing entry !
	// 1) expects id for WHERE, and newEmail and newPassword as values that will be modified.
	// 2) also, returns the 'user' entry from DB.
	// essentially, this UPDATE query is modified to return result via ...RETURNING id, email, created_at, updated_at*
	updatedUser, err := cfg.db.UpdateUser(req.Context(), database.UpdateUserParams{
		ID:             user.ID,
		Email:          newEmail,
		HashedPassword: newHashedPassword,
	})
	if err != nil {
		_respondWithError(w, http.StatusBadRequest, "User does not exist")
		return
	}

	// Happy path - Creating the HTTP Response JSON body with updated entry from 'users' table in DB.
	_respondWithJSON(w, http.StatusOK, UserResponse{
		ID:        updatedUser.ID,
		CreatedAt: updatedUser.CreatedAt,
		UpdatedAt: updatedUser.UpdatedAt,
		Email:     updatedUser.Email,
	})
	// --- Database operation -- end of happy path --
	return
}

// Censoring feature to avoid profanities, when calling HTTP Request -> POST /api/chirp
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

// Helper functions to Marshal HTTP Response into JSON body
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
