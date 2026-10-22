package main

import "net/http"

func newHTTPServer(d *daemon) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /containers", d.handleRunContainer)
	mux.HandleFunc("GET /containers/{id}", d.handleGetContainer)

	return &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}
}
