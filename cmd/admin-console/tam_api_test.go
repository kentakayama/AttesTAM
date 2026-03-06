/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/AttesTAM/internal/domain/model"
	"github.com/kentakayama/AttesTAM/internal/tam"
)

func TestFetchTAMDevicesCBOR(t *testing.T) {
	resetTAMDeviceCachesForTest()

	statusRaw := []tam.AgentStatusRecord{
		{
			AgentKID: []byte("dev-1"),
			Status: tam.AgentStatus{
				Attributes: tam.AgentAttributes{DeviceUEID: []byte{0x10}},
				SuitManifests: []model.SuitManifestOverview{
					{
						TrustedComponentID: mustMarshalCBOR(t, []any{[]byte("app-1")}),
						SequenceNumber:     1,
					},
				},
			},
		},
	}
	statusBody, err := cbor.Marshal(statusRaw)
	if err != nil {
		t.Fatalf("marshal cbor: %v", err)
	}
	listRaw := []tam.AgentStatusKey{
		{
			AgentKID:  []byte("dev-1"),
			UpdatedAt: time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC),
		},
	}
	listBody, err := cbor.Marshal(listRaw)
	if err != nil {
		t.Fatalf("marshal list cbor: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/AgentService/ListAgents":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if len(b) != 0 {
				t.Fatalf("expected empty body, got %q", string(b))
			}
			w.Header().Set("Content-Type", "application/cbor")
			_, _ = w.Write(listBody)
		case "/AgentService/GetAgentStatus":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			var kids [][]byte
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if err := cbor.Unmarshal(b, &kids); err != nil {
				t.Fatalf("decode request cbor: %v", err)
			}
			if len(kids) != 1 || string(kids[0]) != "dev-1" {
				t.Fatalf("unexpected kids: %+v", kids)
			}
			w.Header().Set("Content-Type", "application/cbor")
			_, _ = w.Write(statusBody)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	agents, err := fetchTAMDevices(srv.URL)
	if err != nil {
		t.Fatalf("fetchTAMDevices: %v", err)
	}
	if len(agents) != 1 ||
		!bytes.Equal(agents[0].KID, []byte("dev-1")) ||
		!bytes.Equal([]byte(agents[0].Attributes.Ueid), []byte{0x10}) ||
		!agents[0].LastUpdate.Equal(time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected agents: %+v", agents)
	}
}

func TestFetchTAMDevicesNonCBORResponse(t *testing.T) {
	resetTAMDeviceCachesForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/AgentService/ListAgents" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([][]any{
			{"dev-json", "2026-02-18T11:00:00Z"},
		})
	}))
	defer srv.Close()

	_, err := fetchTAMDevices(srv.URL)
	if err == nil {
		t.Fatal("expected error for non-CBOR response, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported response Content-Type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchTAMDevicesDeltaByUpdatedAt(t *testing.T) {
	resetTAMDeviceCachesForTest()

	type call struct {
		kids []string
	}
	var (
		listCount   int
		statusCalls []call
	)

	statusForKID := map[string]tam.AgentStatusRecord{
		"dev-1": {
			AgentKID: []byte("dev-1"),
			Status: tam.AgentStatus{
				Attributes: tam.AgentAttributes{DeviceUEID: []byte{0x11}},
				SuitManifests: []model.SuitManifestOverview{
					{TrustedComponentID: mustMarshalCBOR(t, []any{[]byte("app-1")}), SequenceNumber: 1},
				},
			},
		},
		"dev-2": {
			AgentKID: []byte("dev-2"),
			Status: tam.AgentStatus{
				Attributes: tam.AgentAttributes{DeviceUEID: []byte{0x22}},
				SuitManifests: []model.SuitManifestOverview{
					{TrustedComponentID: mustMarshalCBOR(t, []any{[]byte("app-2")}), SequenceNumber: 2},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/AgentService/ListAgents":
			listCount++
			var listRaw []tam.AgentStatusKey
			switch listCount {
			case 1:
				listRaw = []tam.AgentStatusKey{
					{AgentKID: []byte("dev-1"), UpdatedAt: time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)},
					{AgentKID: []byte("dev-2"), UpdatedAt: time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)},
				}
			case 2:
				listRaw = []tam.AgentStatusKey{
					{AgentKID: []byte("dev-1"), UpdatedAt: time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)},
					{AgentKID: []byte("dev-2"), UpdatedAt: time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)},
				}
			default:
				listRaw = []tam.AgentStatusKey{
					{AgentKID: []byte("dev-1"), UpdatedAt: time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)},
					{AgentKID: []byte("dev-2"), UpdatedAt: time.Date(2026, 2, 18, 11, 0, 0, 0, time.UTC)},
				}
			}
			listBody, err := cbor.Marshal(listRaw)
			if err != nil {
				t.Fatalf("marshal list cbor: %v", err)
			}
			w.Header().Set("Content-Type", "application/cbor")
			_, _ = w.Write(listBody)
		case "/AgentService/GetAgentStatus":
			var kids [][]byte
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if err := cbor.Unmarshal(b, &kids); err != nil {
				t.Fatalf("decode request cbor: %v", err)
			}
			kidStrings := make([]string, 0, len(kids))
			records := make([]tam.AgentStatusRecord, 0, len(kids))
			for _, kid := range kids {
				k := string(kid)
				kidStrings = append(kidStrings, k)
				records = append(records, statusForKID[k])
			}
			statusCalls = append(statusCalls, call{kids: kidStrings})

			statusBody, err := cbor.Marshal(records)
			if err != nil {
				t.Fatalf("marshal status cbor: %v", err)
			}
			w.Header().Set("Content-Type", "application/cbor")
			_, _ = w.Write(statusBody)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	agents, err := fetchTAMDevices(srv.URL)
	if err != nil {
		t.Fatalf("fetchTAMDevices first: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("unexpected agents length: %d", len(agents))
	}

	agents, err = fetchTAMDevices(srv.URL)
	if err != nil {
		t.Fatalf("fetchTAMDevices second: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("unexpected agents length: %d", len(agents))
	}

	agents, err = fetchTAMDevices(srv.URL)
	if err != nil {
		t.Fatalf("fetchTAMDevices third: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("unexpected agents length: %d", len(agents))
	}

	if len(statusCalls) != 2 {
		t.Fatalf("expected 2 GetAgentStatus calls, got %d", len(statusCalls))
	}
	if !slices.Equal(statusCalls[0].kids, []string{"dev-1", "dev-2"}) {
		t.Fatalf("unexpected first call kids: %+v", statusCalls[0].kids)
	}
	if !slices.Equal(statusCalls[1].kids, []string{"dev-2"}) {
		t.Fatalf("unexpected second call kids: %+v", statusCalls[1].kids)
	}
}

func TestFetchTAMManifestsCBOR(t *testing.T) {
	scid, err := cbor.Marshal([]any{[]byte("m-a")})
	if err != nil {
		t.Fatalf("marshal scid: %v", err)
	}
	raw := []any{
		[]any{scid, uint64(9)},
	}
	body, err := cbor.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal manifests cbor: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/SUITManifestService/ListManifests" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if len(b) != 0 {
			t.Fatalf("expected empty body, got %q", string(b))
		}
		w.Header().Set("Content-Type", "application/cbor")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	manifests, err := fetchTAMManifests(srv.URL)
	if err != nil {
		t.Fatalf("fetchTAMManifests: %v", err)
	}
	if len(manifests) != 1 || componentIDDisplayName(manifests[0].Name) != "['m-a']" || manifests[0].Version != 9 {
		t.Fatalf("unexpected manifests: %+v", manifests)
	}
}

func TestPostTAMManifest(t *testing.T) {
	const expectedBody = "manifest-bytes"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/SUITManifestService/RegisterManifest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/suit-envelope+cose" {
			t.Fatalf("unexpected content type: %s", ct)
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(b) != expectedBody {
			t.Fatalf("unexpected body: %q", string(b))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "example.suit")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte(expectedBody)); err != nil {
		t.Fatalf("part write: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/manifests/register", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	if err := postTAMManifest(rec, req, srv.URL); err != nil {
		t.Fatalf("postTAMManifest: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response json: %v", err)
	}
	if success, _ := resp["success"].(bool); !success {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestPostTAMManifestNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid manifest"))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "bad.suit")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = part.Write([]byte("bad"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/manifests/register", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	err = postTAMManifest(rec, req, srv.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "status 400 from TAM API") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustMarshalCBOR(t *testing.T, v any) []byte {
	t.Helper()
	b, err := cbor.Marshal(v)
	if err != nil {
		t.Fatalf("marshal cbor: %v", err)
	}
	return b
}
