/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestTAMAPILoggingRoundTripperLogsRequestAndResponse(t *testing.T) {
	reqBody, err := cbor.Marshal([][]byte{[]byte("dev-1")})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	respBody, err := cbor.Marshal([][]byte{[]byte("dev-1"), []byte("dev-2")})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, reqBody) {
			t.Fatalf("unexpected request body: %x", body)
		}
		w.Header().Set("Content-Type", "application/cbor")
		_, _ = w.Write(respBody)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	origWriter := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(origWriter)
	defer log.SetFlags(origFlags)

	client := &http.Client{
		Transport: tamAPILoggingRoundTripper{next: http.DefaultTransport},
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/AgentService/GetAgentStatus", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	logged := buf.String()
	if !strings.Contains(logged, "TAM API request:") {
		t.Fatalf("missing request log: %s", logged)
	}
	if !strings.Contains(logged, "POST "+srv.URL+"/AgentService/GetAgentStatus") {
		t.Fatalf("missing request URL: %s", logged)
	}
	if !strings.Contains(logged, "status=200 OK") {
		t.Fatalf("missing response status: %s", logged)
	}
	if !strings.Contains(logged, "h'6465762d31'") {
		t.Fatalf("missing CBOR diagnostic output: %s", logged)
	}
}

func TestTAMAPILoggingRoundTripperRedactsRegisterManifestRequestBody(t *testing.T) {
	reqBody := []byte{0xd8, 0x6b, 0xa1, 0x01, 0x02}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/SUITManifestService/RegisterManifest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, reqBody) {
			t.Fatalf("unexpected request body: %x", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	origWriter := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(origWriter)
	defer log.SetFlags(origFlags)

	client := &http.Client{
		Transport: tamAPILoggingRoundTripper{next: http.DefaultTransport},
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/SUITManifestService/RegisterManifest", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req = req.WithContext(context.WithValue(req.Context(), tamAPIDebugFilenameContextKey, "text.1.envelope.cbor"))
	req.Header.Set("Content-Type", "application/suit-envelope+cose")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	logged := buf.String()
	if !strings.Contains(logged, "POST "+srv.URL+"/SUITManifestService/RegisterManifest") {
		t.Fatalf("missing request URL: %s", logged)
	}
	if !strings.Contains(logged, "body:\n  <omitted: text.1.envelope.cbor>") {
		t.Fatalf("expected request body to be replaced with filename: %s", logged)
	}
	if strings.Contains(logged, "d86ba10102") {
		t.Fatalf("request body leaked into logs: %s", logged)
	}
}
