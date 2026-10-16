package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

func (srv *server) requireCertificate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil ||
			len(r.TLS.VerifiedChains) == 0 ||
			len(r.TLS.VerifiedChains[0]) == 0 {
			srv.writeErrorJSON(
				w, http.StatusUnauthorized, "authentication required",
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (srv *server) requireGroup(group string, next http.Handler) http.Handler {
	return srv.requireCertificate(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			cert := r.TLS.VerifiedChains[0][0]
			if !slices.Contains(cert.Subject.Organization, group) {
				srv.writeErrorJSON(
					w, http.StatusForbidden, "permission denied",
				)
				return
			}

			next.ServeHTTP(w, r)
		},
	))
}

func (srv *server) adminOnly(next http.Handler) http.Handler {
	return srv.requireGroup("hambo:admins", next)
}

func (srv *server) nodeOnly(next http.Handler) http.Handler {
	return srv.requireGroup("hambo:nodes", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			cert := r.TLS.VerifiedChains[0][0]

			nodeID, ok := strings.CutPrefix(
				cert.Subject.CommonName,
				"hambo:node:",
			)
			if !ok || nodeID == "" {
				srv.writeErrorJSON(
					w, http.StatusForbidden, "invalid node identity",
				)
				return
			}

			ctx := context.WithValue(
				r.Context(),
				nodeIDContextKey{},
				nodeID,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		},
	))
}
