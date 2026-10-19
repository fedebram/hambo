package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	publicapi "github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/errdefs"
	"github.com/fedebram/hambo/internal/node"
	"github.com/fedebram/hambo/internal/pki"
	"github.com/fedebram/hambo/internal/store"
	"github.com/fedebram/hambo/internal/token"
)

const maxJoinNodeRequestSize = 16 * 1024

// TODO: all the business logic is inside the handler. For now it is ok.
// Later we can move it into a service layer.
//
// the idea of the join handler is:
// the worker sends its CSR and the requested node name in the body.
// it sends the secret part of the token in the authorization bearer header.
//
// the server retrieves the stored token using the hash of the secret.
// the token must not be expired, both for a first enrollment and for a retry.
//
// on the first enrollment, the server generates a node id, signs the CSR,
// and saves the node together with the enrollment result in one transaction.
// then we return the certificate and node id.
//
// the stored enrollment result contains the node id, the issued certificate,
// and a hash of the original request (the name and CSR).
// this makes the token usable for only one enrollment.
//
// if the worker doesn't receive the response, it can call join again.
// the retry must contain the same original name and CSR.
// if the request hash matches and the node still exists, we return the saved
// certificate and node id. We don't create another node or issue another cert.
//
// retries are allowed only until the token expires. We use a single deadline
// for enrollment and recovery to keep the flow simple.
//
// if the worker never receives its certificate before that deadline, it can
// no longer recover it using this token. The registered node remains.
//
// we don't delete the node automatically when the token expires:
// the server can't know whether the worker lost the response or saved its
// credentials and is simply offline. The admin can delete abandoned nodes.
//
// I think this entire approach can be easy to understand without the need of a two step flow (issue cert and then register).
// here the token of course can be used only for one enrollment.

func (srv *server) joinNodeHandler(w http.ResponseWriter, r *http.Request) {
	// getting the authorization header content in a simple way.
	// no need to adhere to the http rfc... we control clients and server.
	authorization := strings.Fields(r.Header.Get("Authorization"))
	if len(authorization) != 2 ||
		!strings.EqualFold(authorization[0], "Bearer") {
		srv.writeErrorJSON(
			w, http.StatusUnauthorized, "join token required",
		)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJoinNodeRequestSize)

	var request publicapi.JoinNodeRequest
	if !srv.readJSON(w, r, &request) {
		return
	}

	if strings.TrimSpace(request.Name) == "" {
		srv.writeValidationErrorJSON(w, map[string]string{
			"name": "must not be empty",
		})
		return
	}
	if len(request.CSR) == 0 {
		srv.writeValidationErrorJSON(w, map[string]string{
			"csr": "must not be empty",
		})
		return
	}

	presented := token.Token{Secret: authorization[1]}
	secretHash := presented.SecretHash()

	requestHash, err := joinNodeRequestHash(request)
	if err != nil {
		srv.logger.Error("hash join request", "error", err)
		srv.writeErrorJSON(
			w, http.StatusInternalServerError, "could not process enrollment",
		)
		return
	}

	var response publicapi.JoinNodeResponse
	statusCode := http.StatusOK
	var errorMessage string

	err = srv.db.WriteTx(func(tx *bolt.Tx) error {
		metadata, err := srv.tokens.GetTx(tx, secretHash)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				errorMessage = "invalid join token"
				return errdefs.ErrUnauthenticated
			}
			return fmt.Errorf("look up join token: %w", err)
		}

		now := time.Now().UTC()
		if metadata.Expired(now) {
			errorMessage = "join token expired"
			return errdefs.ErrUnauthenticated
		}

		if metadata.Enrollment != nil {
			enrollment := metadata.Enrollment
			if enrollment.RequestHash != requestHash {
				errorMessage = "join token already used for a different request"
				return errdefs.ErrConflict
			}

			if _, err := srv.nodes.GetTx(tx, enrollment.NodeID); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					errorMessage = "node no longer exists"
					return errdefs.ErrNotFound
				}
				return fmt.Errorf("look up enrolled node: %w", err)
			}

			response = publicapi.JoinNodeResponse{
				NodeID:      enrollment.NodeID,
				Certificate: enrollment.Certificate,
			}
			return nil
		}

		nodeID := node.GenerateNodeID()

		certificate, err := srv.ca.IssueNodeCertificate(now, nodeID, request.CSR)
		if err != nil {
			if errors.Is(err, pki.ErrInvalidCSR) {
				errorMessage = "must contain a valid Ed25519 CSR"
				return errdefs.ErrInvalidArgument
			}
			return fmt.Errorf("issue node certificate: %w", err)
		}

		if err := srv.nodes.CreateTx(tx, nodeID, node.Node{
			ID:   nodeID,
			Name: request.Name,
		}); err != nil {
			return fmt.Errorf("create node: %w", err)
		}

		metadata.Enrollment = &token.EnrollmentResult{
			RequestHash: requestHash,
			NodeID:      nodeID,
			Certificate: string(pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: certificate.Raw,
			})),
		}
		if err := srv.tokens.UpdateTx(tx, secretHash, metadata); err != nil {
			return fmt.Errorf("save enrollment result: %w", err)
		}

		response = publicapi.JoinNodeResponse{
			NodeID:      metadata.Enrollment.NodeID,
			Certificate: metadata.Enrollment.Certificate,
		}
		statusCode = http.StatusCreated
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errdefs.ErrUnauthenticated):
			srv.writeErrorJSON(w, http.StatusUnauthorized, errorMessage)
		case errors.Is(err, errdefs.ErrConflict):
			srv.writeErrorJSON(w, http.StatusConflict, errorMessage)
		case errors.Is(err, errdefs.ErrNotFound):
			srv.writeErrorJSON(w, http.StatusGone, errorMessage)
		case errors.Is(err, errdefs.ErrInvalidArgument):
			srv.writeValidationErrorJSON(w, map[string]string{
				"csr": errorMessage,
			})
		default:
			srv.logger.Error("enroll node", "error", err)
			srv.writeErrorJSON(
				w, http.StatusInternalServerError, "could not process enrollment",
			)
		}
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	srv.writeJSON(w, statusCode, response)
}

func joinNodeRequestHash(request publicapi.JoinNodeRequest) (string, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode join request: %w", err)
	}

	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (srv *server) watchNodeHandler(w http.ResponseWriter, r *http.Request) {
	srv.writeErrorJSON(
		w, http.StatusNotImplemented, "node watch is not implemented",
	)
}

func (srv *server) reportNodeStatusHandler(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := nodeIDFromContext(r.Context())
	if !ok {
		srv.logger.Error("node identity missing from request context")
		srv.writeErrorJSON(
			w, http.StatusInternalServerError,
			"could not process node status",
		)
		return
	}

	err := srv.nodes.Modify(nodeID, func(current *node.Node) error {
		now := time.Now().UTC()
		current.LastSeenAt = &now
		return nil
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			srv.writeErrorJSON(
				w, http.StatusNotFound, "node not found",
			)
			return
		}

		srv.logger.Error(
			"record node heartbeat",
			"node_id", nodeID,
			"error", err,
		)
		srv.writeErrorJSON(
			w, http.StatusInternalServerError,
			"could not record node heartbeat",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
