package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRegisterManifestNoTAMReturnsJSON(t *testing.T) {
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
	if err := mw.WriteField("version", "7"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/console/register-tc", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	handleRegisterManifest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Fatalf("unexpected Content-Disposition: %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("unexpected content type: %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"ok\": true") {
		t.Fatalf("expected ok response, got: %s", body)
	}
}
