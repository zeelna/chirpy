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

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	// The idea is that instead of running the increment immediately,
	// you delay it by wrapping it in a function that runs later (when a request actually arrives).
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("request received!") // runs on each request
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, req *http.Request) {
	count := fmt.Sprintf("%v", cfg.fileserverHits.Load()) // convert int[32] into a string
	countMsg := fmt.Sprintf("Hits: %v\n", count)          // prefix such that returns 'Hits: x'

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(countMsg)); err != nil {
		log.Fatal(err)
	}
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits = atomic.Int32{}
	count := fmt.Sprintf("%v", cfg.fileserverHits.Load()) // convert int[32] into a string

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(count + "\n")); err != nil {
		log.Fatal(err)
	}
}

type requestHandlers struct {
	allowed map[string]func(w http.ResponseWriter, req *http.Request)
}

func (rh *requestHandlers) register(name string, fn func(http.ResponseWriter, *http.Request)) {
	if _, ok := (*rh).allowed[name]; ok {
		fmt.Println("error, already exists")
	}
	(*rh).allowed[name] = fn
}

func main() {
	// Strategy pattern
	handlers := requestHandlers{
		allowed: make(map[string]func(w http.ResponseWriter, req *http.Request)),
	}
	handlers.register("/metrics", handlerMetrics)
	handlers.register("/healthz", handlerHealth)
	handlers.register("/reset", handlerReset)

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	// When you register "/" with http.FileServer(http.Dir(".")), it serves all files under the current directory — including assets/logo.png.
	// The file server path should be /app/ (with trailing slash), not /app
	// --------------------------------------------------------
	serverMux := http.NewServeMux()
	// GET /
	serverMux.Handle("/app/",
		apiCfg.middlewareMetricsInc(
			http.StripPrefix("/app", http.FileServer(http.Dir(current_directory)))))
	// Now that we've added a new handler (for path '/healthz' , we don't want potential conflicts with the fileserver handler.)
	// Updated the fileserver to use the /app/ path instead of /. And for that, we used http.StripPrefix inside of serverMux.Handle() function call
	// --------------------------------------------------------

	// GET /healthz
	// create 'readiness endpoint' for Chirpy server.
	// Readiness endpoints are commonly used by external systems to check if our server is ready to receive traffic.
	serverMux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK\n")); err != nil {
			log.Fatal(err)
		}
		// The endpoint should simply return a 200 OK status code indicating that it has started up successfully and is listening for traffic
	}) // Later this endpoint can be enhanced to return a 503 Service Unavailable status code if the server is not ready.
	// --------------------------------------------------------

	// GET /metrics
	// motivation: // how many requests are being made to serve our homepage - in essence, they want to know how many people are viewing the site!
	serverMux.HandleFunc("/metrics", apiCfg.handlerMetrics) // return the count as plain text in the response body.
	// --------------------------------------------------------

	// GET /metrics
	serverMux.HandleFunc("/reset", apiCfg.handlerReset)
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

// you can compile a binary and run server (in the background):
// go build -o out && ./out
// note: Ctrl + C terminates the server.

// Test
// 1. curl http://localhost:8080/
// 2. curl http://localhost:8080/assets/logo.png
//
