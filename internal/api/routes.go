package api

func (srv *server) addRoutes() {
	srv.mux.HandleFunc("GET /health", srv.healthHandler)
	srv.mux.HandleFunc("POST /containers", srv.createContainerHandler)
}
