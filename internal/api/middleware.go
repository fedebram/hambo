package api

import (
	"net/http"
	"slices"
)

func (srv *server) requireGroup(group string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// right now we enforce on the tls config that the client cert is required, but this check
		// is done to prevent panic
		if r.TLS == nil ||
			len(r.TLS.VerifiedChains) == 0 ||
			len(r.TLS.VerifiedChains[0]) == 0 {
			srv.writeErrorJSON(
				w, http.StatusUnauthorized, "authentication required",
			)
			return
		}

		cert := r.TLS.VerifiedChains[0][0]
		if !slices.Contains(cert.Subject.Organization, group) {
			srv.writeErrorJSON(
				w, http.StatusForbidden, "permission denied",
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (srv *server) adminOnly(next http.Handler) http.Handler {
	return srv.requireGroup("hambo:admins", next)
}
