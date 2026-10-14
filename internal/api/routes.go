package api

import "net/http"

func (srv *server) addRoutes() {
	srv.mux.HandleFunc("GET /health", srv.healthHandler)
	srv.mux.Handle("POST /tokens", srv.adminOnly(http.HandlerFunc(srv.createTokenHandler)))
}
