package main

import (
	"log"
	"net/http"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/container"
)

func main() {
	store := container.NewMemoryStore()
	srv := api.NewServer(store)
	log.Fatal(http.ListenAndServe(":4000", srv))
}
