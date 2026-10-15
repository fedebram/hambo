package api

import "net/http"

// mmmmh. it works... would be nice to have the DX of chi...

func (srv *server) addRoutes() {
	srv.mux.HandleFunc("GET /ca", srv.getCAHandler)

	srv.mux.HandleFunc("POST /nodes/join", srv.joinNodeHandler)

	srv.mux.Handle("GET /health",
		srv.requireCertificate(http.HandlerFunc(srv.healthHandler)))

	srv.mux.Handle("POST /tokens",
		srv.adminOnly(http.HandlerFunc(srv.createTokenHandler)))

	// fallback. we need this because we verify the client cert only if it is given.
	// this way some routes (like /ca) can work on normal https (no mtls).
	srv.mux.Handle("/",
		srv.requireCertificate(http.NotFoundHandler()))
}
