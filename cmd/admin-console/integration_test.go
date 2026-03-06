/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kentakayama/tam-over-http/internal/config"
	tamserver "github.com/kentakayama/tam-over-http/internal/server"
)

type integrationAgent struct {
	KID             string                        `json:"kid"`
	LastUpdate      string                        `json:"last_update"`
	Attributes      integrationAgentAttribute     `json:"attribute"`
	InstalledTCList []integrationTrustedComponent `json:"installed-tc"`
}

type integrationAgentAttribute struct {
	UEID string `json:"ueid"`
}

type integrationTrustedComponent struct {
	Name    string `json:"name"`
	Version uint64 `json:"version"`
}

func TestAdminConsoleIntegrationViewManagedDevices(t *testing.T) {
	resetTAMDeviceCachesForTest()

	tamBase := startRealTAMServer(t)
	console := startAdminConsoleServer(t, tamBase)

	resp, err := http.Get(console.URL + "/console/view-managed-devices")
	if err != nil {
		t.Fatalf("GET devices: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status: %d body=%s", resp.StatusCode, body)
	}

	var agents []integrationAgent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		t.Fatalf("decode devices json: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	agent := agents[0]
	if agent.KID != "dummy-teep-agent-kid-for-dev-123" {
		t.Fatalf("unexpected agent kid: %q", agent.KID)
	}
	if agent.LastUpdate == "" {
		t.Fatal("expected last_update to be populated")
	}
	if agent.Attributes.UEID == "" {
		t.Fatal("expected attribute.ueid to be populated")
	}
	if len(agent.InstalledTCList) == 0 {
		t.Fatal("expected installed-tc to be populated")
	}
	if agent.InstalledTCList[0].Name != "['hello.txt']" {
		t.Fatalf("unexpected TC name: %q", agent.InstalledTCList[0].Name)
	}
}

func TestAdminConsoleIntegrationViewManagedTCs(t *testing.T) {
	tamBase := startRealTAMServer(t)
	console := startAdminConsoleServer(t, tamBase)

	resp, err := http.Get(console.URL + "/console/view-managed-tcs")
	if err != nil {
		t.Fatalf("GET manifests: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status: %d body=%s", resp.StatusCode, body)
	}

	var manifests []integrationTrustedComponent
	if err := json.NewDecoder(resp.Body).Decode(&manifests); err != nil {
		t.Fatalf("decode manifests json: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0].Name != "['hello.txt']" {
		t.Fatalf("unexpected manifest name: %q", manifests[0].Name)
	}
	if manifests[0].Version != 0 {
		t.Fatalf("unexpected manifest version: %d", manifests[0].Version)
	}
}

func TestAdminConsoleIntegrationRegisterTC(t *testing.T) {
	tamBase := startRealTAMServer(t)
	console := startAdminConsoleServer(t, tamBase)

	manifestBytes, err := os.ReadFile(resolvePath(filepath.Join("..", "..", "doc", "examples", "text.1.envelope.cbor")))
	if err != nil {
		t.Fatalf("read manifest fixture: %v", err)
	}

	var reqBody bytes.Buffer
	writer := multipart.NewWriter(&reqBody)
	part, err := writer.CreateFormFile("file", "text.1.envelope.cbor")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(manifestBytes); err != nil {
		t.Fatalf("part write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, console.URL+"/console/register-tc", &reqBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST register-tc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status: %d body=%s", resp.StatusCode, body)
	}

	var registerResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if success, _ := registerResp["success"].(bool); !success {
		t.Fatalf("unexpected register response: %+v", registerResp)
	}

	manifests := fetchManifestList(t, console.URL)
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest after register, got %d", len(manifests))
	}
	if manifests[0].Name != "['hello.txt']" {
		t.Fatalf("unexpected manifest name after register: %q", manifests[0].Name)
	}
	if manifests[0].Version != 1 {
		t.Fatalf("expected manifest version 1 after register, got %d", manifests[0].Version)
	}
}

func TestAdminConsoleIntegrationUpstreamFailureReturnsBadGateway(t *testing.T) {
	prev := conf
	conf = AppConfig{TAMAPIBase: "http://127.0.0.1:1"}
	t.Cleanup(func() { conf = prev })

	req := httptest.NewRequest(http.MethodGet, "/console/view-managed-devices", nil)
	rec := httptest.NewRecorder()

	handleListAgents(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func startAdminConsoleServer(t *testing.T, tamBase string) *httptest.Server {
	t.Helper()

	prev := conf
	conf = AppConfig{TAMAPIBase: tamBase}
	t.Cleanup(func() {
		conf = prev
		resetTAMDeviceCachesForTest()
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/console/view-managed-devices", handleListAgents)
	mux.HandleFunc("/console/view-managed-tcs", handleListManifestsService)
	mux.HandleFunc("/console/register-tc", handleRegisterManifest)

	srv := httptest.NewServer(withCORS(mux))
	t.Cleanup(srv.Close)
	return srv
}

func startRealTAMServer(t *testing.T) string {
	t.Helper()

	addr := reserveTCPAddr(t)
	srv, err := tamserver.New(config.TAMConfig{
		Addr:             addr,
		InsecureDemoMode: true,
		Logger:           log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("tamserver.New: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	waitForTAMReady(t, "http://"+addr)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Fatalf("tam shutdown: %v", err)
		}
		if err := <-errCh; err != nil {
			t.Fatalf("tam listen: %v", err)
		}
	})

	return "http://" + addr
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func waitForTAMReady(t *testing.T, base string) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, base+"/AgentService/ListAgents", nil)
		if err != nil {
			t.Fatalf("NewRequest readiness: %v", err)
		}
		req.Header.Set("Accept", "application/cbor")

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("TAM server did not become ready: %s", base)
}

func fetchManifestList(t *testing.T, consoleURL string) []integrationTrustedComponent {
	t.Helper()

	resp, err := http.Get(consoleURL + "/console/view-managed-tcs")
	if err != nil {
		t.Fatalf("GET manifests: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected manifest status: %d body=%s", resp.StatusCode, body)
	}

	var manifests []integrationTrustedComponent
	if err := json.NewDecoder(resp.Body).Decode(&manifests); err != nil {
		t.Fatalf("decode manifests json: %v", err)
	}
	return manifests
}
