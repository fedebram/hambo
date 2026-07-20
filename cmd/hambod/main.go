package main

import (
	"log"
	"net/http"

	"github.com/fedebram/hambo/internal/api"
)

func main() {
	srv := api.NewServer()
	log.Fatal(http.ListenAndServe(":4000", srv))
}
