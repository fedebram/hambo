package api

func (srv *server) addRoutes() {
	srv.mux.HandleFunc("GET /health", srv.healthHandler)
	srv.mux.HandleFunc("POST /containers", srv.createContainerHandler)
	srv.mux.HandleFunc("GET /containers/{name}", srv.getContainerHandler)
	srv.mux.HandleFunc("POST /containers/{name}/start", srv.startContainerHandler)
	srv.mux.HandleFunc("POST /containers/{name}/stop", srv.stopContainerHandler)
	srv.mux.HandleFunc("DELETE /containers/{name}", srv.deleteContainerHandler)
}
