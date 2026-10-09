package api

import (
	"net/http"

	publicapi "github.com/fedebram/hambo/api"
)

// TODO: return json error response for method not allowed and page not found (overriding the default mux)

func (srv *server) healthHandler(w http.ResponseWriter, r *http.Request) {
	srv.writeJSON(w, http.StatusOK, publicapi.HealthResponse{
		Status: "ok",
	})
}
