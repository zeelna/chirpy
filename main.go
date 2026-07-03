package main

import (
	"log"
	"net/http"
)

const (
	project_root_path = "/"
	current_directory = "."
	port              = "8080"
)

func main() {

	serverMux := http.NewServeMux()

	// When you register "/" with http.FileServer(http.Dir(".")), it serves all files under the current directory — including assets/logo.png.
	serverMux.Handle(project_root_path, http.FileServer(http.Dir(current_directory)))

	//serverMux.Handle(root_path, http.FileServer(http.Dir(current_directory+"/assets/logo.png")))

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
