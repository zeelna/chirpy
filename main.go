package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

const (
	project_root_path = "/"
	current_directory = "."
	port              = "8080"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

// Heplful Go doc links:
// // https://pkg.go.dev/net/http#ServeMux.Handle
// type Handler: https://pkg.go.dev/net/http#Handler

// https://pkg.go.dev/net/http#ResponseWriter
//https://pkg.go.dev/net/http#ResponseWriter.Write

func main() {
	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
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
	serverMux.HandleFunc("GET /healthz", apiCfg.handlerHealth) // Later this endpoint can be enhanced to return a 503 Service Unavailable status code if the server is not ready.
	// --------------------------------------------------------
	// GET /metrics -- how many people are viewing the site (until server is turned off)
	// motivation: // how many requests are being made to serve our homepage - in essence, they want to know
	serverMux.HandleFunc("GET /metrics", apiCfg.handlerMetrics) // return the count as plain text in the response body.
	// --------------------------------------------------------

	// GET /metrics -- reset to '0' many people are viewing the site!
	serverMux.HandleFunc("POST /reset", apiCfg.handlerReset)
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Status Code: Send HTTP 200/ok
	w.WriteHeader(http.StatusOK)

	// Send 'Hits: <cfg.fileserverHits>' in response body
	count := fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load()) // convert int[32] into a string
	if _, err := w.Write([]byte(count)); err != nil {
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

// you can compile a binary and run server (in the background):
// go build -o out && ./out
// note: Ctrl + C terminates the server.

// Test
// 1. curl http://localhost:8080/
// 2. curl http://localhost:8080/assets/logo.png
//
