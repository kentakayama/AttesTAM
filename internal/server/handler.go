/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
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
	"strconv"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

const (
	maxRequestBodyBytes        = 32 << 10 // 32 KiB should cover all test vectors in general.
	maxRequestBodyBytesForSUIT = 32 << 20 // 32 MiB should cover all test vectors for SUIT.
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
	case "/SUITManifestService/ListManifests":
		h.getManifests(w, r)
	case "/SUITManifestService/RegisterManifest":
		h.addTCManifest(w, r)
	case "/AgentService/ListAgents":
		h.getAgentList(w, r)
	case "/AgentService/GetAgentStatus":
		h.getAgentStatus(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *handler) tamOverHttp(w http.ResponseWriter, r *http.Request) {
	// NOTE: authentication is done in the TEEP Protocol layer
	if r.Method != http.MethodPost {
		// TODO: should be 405, but currently returns 500 respecting draft-ietf-teep-otrp-over-http which allows only 5xx for error responses. --- IGNORE ---
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// check the content
	if r.Header.Get("Accept") != "application/teep+cbor" {
		h.logger.Printf("content type mismatch: expected application/teep+cbor, actual %v", r.Header.Get("Accept"))
		http.Error(w, "This endpoint only accepts Accept: application/teep+cbor", http.StatusUnsupportedMediaType)
		return
	}
	if r.Header.Get("Content-Type") != "application/teep+cbor" {
		h.logger.Printf("content type mismatch: expected application/teep+cbor, actual %v", r.Header.Get("Content-Type"))
		http.Error(w, "This endpoint only accepts Content-Type: application/teep+cbor", http.StatusUnsupportedMediaType)
		return
	}

	// protect against overly large bodies; MaxBytesReader will return http.ErrBodyTooLarge
	// if the body is bigger than the limit, and also write a 413 if the handler doesn't
	// consume all of it.
	// limit size; ReadAll will return either http.ErrBodyTooLarge or *http.MaxBytesError
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			h.logger.Printf("request body is too large: %v", err)
			// TODO: should be 413, but currently returns 500 respecting draft-ietf-teep-otrp-over-http which allows only 5xx for error responses. --- IGNORE ---
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		h.logger.Printf("failed reading request body: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := r.Body.Close(); err != nil {
		h.logger.Printf("failed to close request body: %v", err)
		// TODO: should be 400, but currently returns 500 respecting draft-ietf-teep-otrp-over-http which allows only 5xx for error responses. --- IGNORE ---
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	responseBody, err := h.tam.ResolveTEEPMessage(body)
	var resp responseSpec
	if err != nil {
		// TODO: distinguish different types of errors and return appropriate status codes and messages.
		h.logger.Printf("Internal Server Error occurred: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(responseBody) == 0 {
		resp = responseSpec{
			status: http.StatusNoContent,
		}
		h.logger.Printf("Returns NoContent")
	} else {
		resp = responseSpec{
			status:      http.StatusOK,
			body:        responseBody,
			contentType: "application/teep+cbor",
		}
		h.logger.Printf("Returns %d bytes message", len(responseBody))
	}
	h.writeResponse(w, resp)
}

func (h *handler) getManifests(w http.ResponseWriter, r *http.Request) {
	// TODO: authenticate and authorize TC Developer to add new SUIT Manifest for the TC

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// check the content
	if r.Header.Get("Accept") != "application/cbor" {
		h.logger.Printf("content type mismatch: expected application/cbor, actual %v", r.Header.Get("Accept"))
		http.Error(w, "This endpoint only accepts Accept: application/cbor", http.StatusUnsupportedMediaType)
		return
	}

	// devName := "developer1@example.com" // get developer id
	// entity, err := h.tam.FindEntity(devName)
	// if err != nil {
	// 	h.logger.Printf("failed to find TC Developer entity: %v", err)
	// 	http.Error(w, "failed to find TC Developer entity", http.StatusBadRequest)
	// 	return
	// }
	tcIDs := [][]byte{
		{0x81, 0x49, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74}, // ['hello.txt']
	}

	var manifests []*model.SuitManifestOverview
	for i := 0; i < len(tcIDs); i++ {
		manifest, err := h.tam.GetManifest(tcIDs[i])
		if err != nil {
			h.logger.Printf("failed to get SUIT Manifest: %v", err)
			http.Error(w, "failed to get SUIT Manifest", http.StatusInternalServerError)
			return
		}
		if manifest == nil {
			h.logger.Printf("SUIT Manifest for TC %v not found", tcIDs[i])
			continue
		}

		overview := model.SuitManifestOverview{
			TrustedComponentID: manifest.TrustedComponentID,
			SequenceNumber:     manifest.SequenceNumber,
		}
		manifests = append(manifests, &overview)
	}

	if len(manifests) == 0 {
		resp := responseSpec{
			status:      http.StatusNoContent,
			contentType: "application/cbor",
		}
		h.writeResponse(w, resp)
		return
	}
	encoded, err := cbor.Marshal(manifests)
	if err != nil {
		h.logger.Printf("failed to encode SUIT Manifests: %v", err)
		http.Error(w, "failed to get SUIT Manifest", http.StatusInternalServerError)
		return
	}

	resp := responseSpec{
		status:      http.StatusOK,
		body:        encoded,
		contentType: "application/cbor",
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

	// for SUIT envelopes we allow a larger limit
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytesForSUIT)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			h.logger.Printf("request body is too large: %v", err)
			http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
			return
		}
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
		switch err {
		case tam.ErrNotAuthenticated:
			http.Error(w, "the manifest signing key is not trusted by the TAM", http.StatusBadRequest)
			return
		case suit.ErrSUITManifestSmallerSequenceNumber:
			http.Error(w, "the manifest sequence number should be bigger than the existing one", http.StatusBadRequest)
			return
		case suit.ErrSUITManifestSigningKeyMismatch:
			http.Error(w, "the manifest signing key should be the same as the existing one", http.StatusBadRequest)
			return
		default:
			http.Error(w, "failed to add SUIT Manifest", http.StatusInternalServerError)
			return
		}
	}

	kid, _ := key.Thumbprint(crypto.SHA256)
	h.logger.Printf("A TC is registed: {Key: h'%s', TC: h'%s', Seq: %d}", hex.EncodeToString(kid), hex.EncodeToString(encodedComponentID), manifest.Value.ManifestSequenceNumber)

	resp := responseSpec{
		status:      http.StatusOK,
		body:        []byte("OK\n"),
		contentType: "text/plain",
	}
	h.writeResponse(w, resp)
}

func (h *handler) getAgentList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// check the content
	if r.Header.Get("Accept") != "application/cbor" {
		h.logger.Printf("content type mismatch: expected application/cbor, actual %v", r.Header.Get("Accept"))
		http.Error(w, "This endpoint only accepts Accept: application/cbor", http.StatusUnsupportedMediaType)
		return
	}

	// TODO: authenticate and authorize the caller to get TEEP Agent status:
	// all TEEP Agent status for TAM Admin, while ones administragted by Device Admin itself.

	// TODO: get the caller's role and ID from the authentication result,
	// and find the corresponding entity in the TAM.
	// For now, we assume the caller is a TAM Admin and use a hardcoded admin name to find the entity.
	adminName := "admin@example.com"
	entity, err := h.tam.FindEntity(adminName)
	if err != nil {
		h.logger.Printf("failed to find Device Admin entity: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var agentList []*tam.AgentStatusKey
	switch {
	case entity.IsTAMAdmin:
		agentList, err = h.tam.GetAgentStatuses(entity)
	case entity.IsDeviceAdmin && entity.ID > 0:
		agentList, err = h.tam.GetAgentStatuses(entity)
	default:
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err != nil {
		h.logger.Printf("failed to get TEEP Agent list: %v", err)
		http.Error(w, "failed to get TEEP Agent list", http.StatusInternalServerError)
		return
	}

	encoded, err := cbor.Marshal(agentList)
	if err != nil {
		h.logger.Printf("failed to encode TEEP Agent list: %v", err)
		http.Error(w, "failed to encode TEEP Agent list", http.StatusInternalServerError)
		return
	}

	resp := responseSpec{
		status:      http.StatusOK,
		body:        encoded,
		contentType: "application/cbor",
	}
	h.writeResponse(w, resp)
}

func (h *handler) getAgentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// check the accept header
	if r.Header.Get("Accept") != "application/cbor" {
		h.logger.Printf("content type mismatch: expected application/cbor, actual %v", r.Header.Get("Accept"))
		http.Error(w, "This endpoint only accepts Accept: application/cbor", http.StatusUnsupportedMediaType)
		return
	}
	if r.Header.Get("Content-Type") != "application/cbor" {
		h.logger.Printf("content type mismatch: expected application/cbor, actual %v", r.Header.Get("Content-Type"))
		http.Error(w, "This endpoint only accepts Content-Type: application/cbor", http.StatusUnsupportedMediaType)
		return
	}

	// similar protection for agent status requests
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			h.logger.Printf("request body is too large: %v", err)
			http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
			return
		}
		h.logger.Printf("failed reading request body: %v", err)
		http.Error(w, "failed to parse Request Body", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		h.logger.Printf("failed to close request body: %v", err)
		http.Error(w, "failed to parse Request Body", http.StatusBadRequest)
		return
	}
	var kids [][]byte
	if err := cbor.Unmarshal(body, &kids); err != nil {
		h.logger.Printf("failed to parse request body as list of KIDs: %v", err)
		http.Error(w, "failed to parse Request Body", http.StatusBadRequest)
		return
	}

	// TODO: get the caller's role and ID from the authentication result,
	// and find the corresponding entity in the TAM.
	// For now, we assume the caller is a TAM Admin and use a hardcoded admin name to find the entity.
	adminName := "admin@example.com"
	entity, err := h.tam.FindEntity(adminName)
	if err != nil {
		h.logger.Printf("failed to find Device Admin entity: %v", err)
		http.Error(w, "failed to find Device Admin entity", http.StatusBadRequest)
		return
	}

	var statusList []*tam.AgentStatusRecord
	for _, kid := range kids {
		agentStatus, err := h.tam.GetAgentStatus(entity, kid)
		if err != nil {
			h.logger.Printf("failed to get TEEP Agent status: %v", err)
			http.Error(w, "failed to get TEEP Agent status", http.StatusInternalServerError)
			return
		}
		if agentStatus == nil {
			continue
		}
		h.logger.Printf("got TEEP Agent status: %+v", agentStatus)
		statusList = append(statusList, agentStatus)
	}

	if len(statusList) == 0 {
		resp := responseSpec{
			status:      http.StatusNoContent,
			contentType: "application/cbor",
		}
		h.writeResponse(w, resp)
		return
	}
	encoded, err := cbor.Marshal(statusList)
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
