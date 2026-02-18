package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestFetchTAMDevicesCBOR(t *testing.T) {
	statusRaw := []any{
		[]any{
			"dev-1",
			map[any]any{
				"attributes":   map[any]any{256: []byte{0x10}},
				"installed-tc": []any{[]any{"app-1", int64(1)}},
			},
		},
	}
	statusBody, err := cbor.Marshal(statusRaw)
	if err != nil {
		t.Fatalf("marshal cbor: %v", err)
	}
	listRaw := []any{
		[]any{"dev-1", "2026-02-18T10:00:00Z"},
	}
	listBody, err := cbor.Marshal(listRaw)
	if err != nil {
		t.Fatalf("marshal list cbor: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/AgentService/ListAgents":
			if r.Method != http.MethodPost {
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
			var kids []string
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if err := cbor.Unmarshal(b, &kids); err != nil {
				t.Fatalf("decode request cbor: %v", err)
			}
			if len(kids) != 1 || kids[0] != "dev-1" {
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
	if len(agents) != 1 || agents[0].KID != "dev-1" || agents[0].Attributes.Ueid != "10" || agents[0].LastUpdate != "2026-02-18T10:00:00Z" {
		t.Fatalf("unexpected agents: %+v", agents)
	}
}

func TestFetchTAMDevicesJSONFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/AgentService/ListAgents":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([][]any{
				{"dev-json", "2026-02-18T11:00:00Z"},
			})
		case "/AgentService/GetAgentStatus":
			var kids []string
			b, _ := io.ReadAll(r.Body)
			_ = cbor.Unmarshal(b, &kids)
			if len(kids) != 1 || kids[0] != "dev-json" {
				t.Fatalf("unexpected kids: %+v", kids)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"kid":          "dev-json",
					"attribute":    map[string]any{"ueid": "u-json"},
					"installed-tc": []map[string]any{{"name": []string{"dw=="}, "ver": 2}},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	agents, err := fetchTAMDevices(srv.URL)
	if err != nil {
		t.Fatalf("fetchTAMDevices: %v", err)
	}
	if len(agents) != 1 || agents[0].KID != "dev-json" {
		t.Fatalf("unexpected agents: %+v", agents)
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
		if r.Method != http.MethodPost {
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
	if len(manifests) != 1 || manifests[0].Name != "m-a" || manifests[0].Ver != 9 {
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
		if ct := r.Header.Get("Content-Type"); ct != "application/octet-stream" {
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
	if err := mw.WriteField("version", "11"); err != nil {
		t.Fatalf("WriteField: %v", err)
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
	if ok, _ := resp["ok"].(bool); !ok {
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
