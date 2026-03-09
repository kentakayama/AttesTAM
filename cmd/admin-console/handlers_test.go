/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleRegisterManifestWithoutTAMReturnsServerError(t *testing.T) {
	prev := conf
	conf = AppConfig{}
	t.Cleanup(func() { conf = prev })

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "sample.suit")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("manifest-body")); err != nil {
		t.Fatalf("part write: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/console/register-tc", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	handleRegisterManifest(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("unexpected content type: %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "tam-api-base is required") {
		t.Fatalf("expected configuration error, got: %s", body)
	}
}

func TestIndexHandlerSetsNoStoreHeadersAndAssetVersion(t *testing.T) {
	prevConf := conf
	prevBuildTime := buildTime
	prevTmpl := tmpl
	conf = AppConfig{TAMAPIBase: "http://127.0.0.1:8080"}
	buildTime = time.Unix(1234567890, 0)
	tmpl = template.Must(template.New("index.html").Parse(`{{.ConnectedTAM}}|{{.AssetVersion}}`))
	t.Cleanup(func() {
		conf = prevConf
		buildTime = prevBuildTime
		tmpl = prevTmpl
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	indexHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("unexpected Cache-Control header: %q", got)
	}
	if got := rec.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("unexpected Pragma header: %q", got)
	}
	if got := rec.Header().Get("Expires"); got != "0" {
		t.Fatalf("unexpected Expires header: %q", got)
	}
	if got := rec.Body.String(); got != "http://127.0.0.1:8080|1234567890" {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestRespondJSONSetsNoStoreHeaders(t *testing.T) {
	rec := httptest.NewRecorder()

	respondJSON(rec, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("unexpected Cache-Control header: %q", got)
	}
	if got := rec.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("unexpected Pragma header: %q", got)
	}
	if got := rec.Header().Get("Expires"); got != "0" {
		t.Fatalf("unexpected Expires header: %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("unexpected content type: %q", ct)
	}
}
