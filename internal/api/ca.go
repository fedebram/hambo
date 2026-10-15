package api

import (
	"encoding/pem"
	"net/http"
)

func (srv *server) getCAHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/pem-certificate-chain")

	if err := pem.Encode(w, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: srv.ca.Certificate.Raw,
	}); err != nil {
		srv.logger.Error("write CA certificate response", "error", err)
	}
}
