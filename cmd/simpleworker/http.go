package main

import "net/http"

func newHTTPServer() *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /containers", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})

	mux.HandleFunc("GET /containers/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})

	return &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}
}
