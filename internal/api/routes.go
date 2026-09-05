package api

func (srv *server) addRoutes() {
	srv.mux.HandleFunc("GET /health", srv.healthHandler)
	srv.mux.HandleFunc("POST /containers", srv.createContainerHandler)
	srv.mux.HandleFunc("GET /containers/{name}", srv.getContainerHandler)
	srv.mux.HandleFunc("POST /containers/{name}/start", srv.startContainerHandler)
	srv.mux.HandleFunc("POST /containers/{name}/stop", srv.stopContainerHandler)
	srv.mux.HandleFunc("DELETE /containers/{name}", srv.deleteContainerHandler)

	if srv.imageService != nil {
		srv.addImageRoutes()
	}
}

func (srv *server) addImageRoutes() {
	srv.mux.HandleFunc("DELETE /images/{name}", srv.deleteImageHandler)
	srv.mux.HandleFunc("GET /images", srv.listImagesHandler)
	srv.mux.HandleFunc("POST /images/pull", srv.pullImageHandler)
}
