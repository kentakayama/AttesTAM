/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package server

import (
	"crypto"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

const (
	maxRequestBodyBytes = 1 << 20 // 1 MiB should cover all test vectors.
)

// TODO: remove these unused variables when implementing attestation payload handling.
var (
	attestationPayloadPath        = filepath.Join("resources", "attestation_payload.bin")
	errAttestationPayloadNotFound = errors.New("attestation payload (TEEP field 7) not found")
)

type handler struct {
	tam    *tam.TAM
	logger *log.Logger
}

type responseSpec struct {
	status      int
	body        []byte
	contentType string
}

func newHandler(tam *tam.TAM, logger *log.Logger) (*handler, error) {
	return &handler{
		tam:    tam,
		logger: logger,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/tam":
		h.tamOverHttp(w, r)
		return
	case "/tc-developer/addManifest":
		h.addTCManifest(w, r)
		return
	case "/admin/getAgents":
		h.getAgentStatusesByTAMAdmin(w, r)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (h *handler) tamOverHttp(w http.ResponseWriter, r *http.Request) {
	// NOTE: authentication is done in the TEEP Protocol layer
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// check the content
	if r.Header.Get("Content-Type") != "application/teep+cbor" {
		h.logger.Printf("content type mismatch: expected application/teep+cbor, actual %v", r.Header.Get("Content-Type"))
		http.Error(w, "This endpoint only accepts Content-Type: application/teep+cbor", http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err != nil {
		h.logger.Printf("failed to read request body: %v", err)
		http.Error(w, "failed to parse TEEP Message", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		h.logger.Printf("failed to close request body: %v", err)
		http.Error(w, "failed to parse TEEP Message", http.StatusBadRequest)
		return
	}

	responseBody, err := h.tam.ResolveTEEPMessage(body)
	var resp responseSpec
	if err != nil {
		resp = responseSpec{
			status: http.StatusInternalServerError,
		}
	} else {
		if len(responseBody) == 0 {
			resp = responseSpec{
				status: http.StatusNoContent,
			}
		} else {
			resp = responseSpec{
				status:      http.StatusOK,
				body:        body,
				contentType: "application/teep+cbor",
			}
		}
	}
	h.writeResponse(w, resp)
}

func (h *handler) addTCManifest(w http.ResponseWriter, r *http.Request) {
	// TODO: authenticate and authorize TC Developer to add new SUIT Manifest for the TC

	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// check the content
	if r.Header.Get("Content-Type") != "application/suit-envelope+cose" {
		h.logger.Printf("content type mismatch: expected application/suit-envelope+cose, actual %v", r.Header.Get("Content-Type"))
		http.Error(w, "This endpoint only accepts Content-Type: application/suit-envelope+cose", http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err != nil {
		h.logger.Printf("failed reading request body: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		h.logger.Printf("failed closing request body: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	// parse the body as SUIT_Envelope or Tagged_SUIT_Envelope
	var envelope suit.Envelope
	if err := cbor.Unmarshal(body, &envelope); err != nil {
		h.logger.Printf("failed to parse SUIT Envelope: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	// get only the untagged one, because TEEP Protocol transfers untagged ones
	_, untaggedEnvelopeBytes := envelope.SkipTag(body)

	key, err := h.tam.GetEntityKey(envelope.AuthenticationWrapper.Value.AuthenticationBlocks[0].KID)
	if err != nil || key == nil {
		h.logger.Printf("the manifest signing key is not trusted by the TAM: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	if err := envelope.Verify(key); err != nil {
		h.logger.Printf("failed to verify the manifest: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	var manifest suit.Nested[suit.Manifest]
	if err := cbor.Unmarshal(envelope.ManifestBstr, &manifest); err != nil {
		h.logger.Printf("failed to parse SUIT Manifest: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	if len(manifest.Value.Common.Value.Components) != 1 {
		h.logger.Printf("the number of Trusted Component should be exactly one: 1 != %d", len(manifest.Value.Common.Value.Components))
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	encodedComponentID, err := cbor.Marshal(manifest.Value.Common.Value.Components[0])
	if err != nil {
		h.logger.Printf("failed to encode ComponentID: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	if err := h.tam.SetEnvelope(untaggedEnvelopeBytes, envelope.AuthenticationWrapper.Value.DigestBstr, envelope.AuthenticationWrapper.Value.AuthenticationBlocks[0].KID, encodedComponentID, manifest.Value.ManifestSequenceNumber); err != nil {
		h.logger.Printf("failed to SetEnvelope: %v", err)
		http.Error(w, "failed to parse SUIT Manifest", http.StatusBadRequest)
		return
	}

	kid, _ := key.Thumbprint(crypto.SHA256)
	h.logger.Printf("A TC is registed: {Key: h'%s', TC: h'%s', Seq: %d}", hex.EncodeToString(kid), hex.EncodeToString(encodedComponentID), manifest.Value.ManifestSequenceNumber)

	resp := responseSpec{
		status:      http.StatusOK,
		body:        []byte("OK"),
		contentType: "text/plain",
	}
	h.writeResponse(w, resp)
}

func (h *handler) getAgentStatusesByTAMAdmin(w http.ResponseWriter, r *http.Request) {
	// TODO: authenticate and authorize TAM Admin to get TEEP Agent status

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// TODO: check the content?

	// check the accept header
	if r.Header.Get("Accept") != "application/cbor" {
		h.logger.Printf("content type mismatch: expected application/cbor, actual %v", r.Header.Get("Accept"))
		http.Error(w, "This endpoint only accepts Accept: application/cbor", http.StatusUnsupportedMediaType)
		return
	}

	adminName := "admin@example.com"
	entity, err := h.tam.FindEntity(adminName)
	if err != nil {
		h.logger.Printf("failed to find Device Admin entity: %v", err)
		http.Error(w, "failed to find Device Admin entity", http.StatusBadRequest)
		return
	}
	agentStatus, err := h.tam.GetAgentStatus(entity, []byte("dummy-teep-agent-kid-for-dev-123"))
	if err != nil {
		h.logger.Printf("failed to get TEEP Agent status: %v", err)
		http.Error(w, "failed to get TEEP Agent status", http.StatusInternalServerError)
		return
	}
	h.logger.Printf("got TEEP Agent status: %+v", agentStatus)

	if agentStatus == nil {
		resp := responseSpec{
			status:      http.StatusNoContent,
			contentType: "application/cbor",
		}
		h.writeResponse(w, resp)
		return
	}
	encoded, err := cbor.Marshal([]*model.AgentStatus{agentStatus})
	if err != nil {
		h.logger.Printf("failed to encode TEEP Agent status: %v", err)
		http.Error(w, "failed to encode TEEP Agent status", http.StatusInternalServerError)
		return
	}

	resp := responseSpec{
		status:      http.StatusOK,
		body:        encoded,
		contentType: "application/cbor",
	}
	h.writeResponse(w, resp)
}

func (h *handler) writeResponse(w http.ResponseWriter, spec responseSpec) {
	w.Header().Set("Server", "Bar/2.2")

	if len(spec.body) > 0 {
		for k, v := range defaultHeaders {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", spec.contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(spec.body)))
		w.WriteHeader(spec.status)
		if _, err := w.Write(spec.body); err != nil {
			h.logger.Printf("failed writing response body: %v", err)
		}
		return
	}

	w.WriteHeader(spec.status)
}

var defaultHeaders = map[string]string{
	"Cache-Control":           "no-store",
	"X-Content-Type-Options":  "nosniff",
	"Content-Security-Policy": "default-src 'none'",
	"Referrer-Policy":         "no-referrer",
}
