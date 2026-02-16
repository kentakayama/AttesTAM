// Admin Console for Building Security Service Provider
// External API integration + config file support (config.json).
// Precedence of settings:
//   1) Environment variables (PORT, EXTERNAL_API_BASE)
//   2) config.json values
//   3) built-in defaults
//
// To run (manual test pattern #3):
//   1) Start mock API:  go run ./cmd/mockapi
//   2) Start app:      go run .
//   3) Open: http://localhost:8080 and click "Show Devices"

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AppState struct {
	mu        sync.RWMutex
	agents    []Agent
	manifests []Manifest
}

// AppConfig holds runtime settings.
type AppConfig struct {
	Server struct {
		Port int `json:"port"`
	} `json:"server"`
	ExternalAPIBase string `json:"externalApiBase"`
}

var (
	tmpl      *template.Template
	state     = &AppState{}
	buildTime = time.Now()
	conf      AppConfig
)

func main() {
	// defaults
	conf.Server.Port = 8080
	conf.ExternalAPIBase = ""

	// Load config.json if present
	loadConfig(resolvePath("config.json"), &conf)

	// Env overrides
	if v := os.Getenv("EXTERNAL_API_BASE"); v != "" {
		conf.ExternalAPIBase = v
	}
	if v := os.Getenv("PORT"); v != "" {
		// Keep as string for ListenAndServe; we still print configured port
		// but not converting for simplicity.
	}

	var err error
	tmpl, err = template.ParseFiles(resolvePath(filepath.Join("templates", "index.html")))
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/api/devices", devicesHandler)
	mux.HandleFunc("/api/manifests", manifestsHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(resolvePath("static")))))

	addr := fmt.Sprintf(":%d", conf.Server.Port)
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	log.Printf("Admin Console listening on http://localhost%v (build: %s) externalApiBase=%q", addr, buildTime.Format(time.RFC3339), conf.ExternalAPIBase)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func loadConfig(path string, out *AppConfig) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewDecoder(f).Decode(out)
}

func resolvePath(rel string) string {
	candidates := []string{
		rel,
		filepath.Join("cmd", "admin-console", rel),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return rel
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func devicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	base := conf.ExternalAPIBase
	if base != "" {
		devices, err := fetchExternalDevices(base)
		if err != nil {
			log.Printf("external fetch failed: %v", err)
			http.Error(w, fmt.Sprintf("external fetch failed: %v", err), http.StatusBadGateway)
			return
		}
		respondJSON(w, devices)
		return
	}

	state.mu.RLock()
	defer state.mu.RUnlock()
	respondJSON(w, state.agents)
}

func manifestsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		base := conf.ExternalAPIBase
		if base != "" {
			manifests, err := fetchExternalManifests(base)
			if err != nil {
				log.Printf("external fetch manifests failed: %v", err)
				http.Error(w, fmt.Sprintf("external fetch failed: %v", err), http.StatusBadGateway)
				return
			}
			respondJSON(w, manifests)
			return
		}
		state.mu.RLock()
		defer state.mu.RUnlock()
		respondJSON(w, state.manifests)
	case http.MethodPost:
		ct := r.Header.Get("Content-Type")
		var m Manifest
		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				http.Error(w, "failed to parse form", http.StatusBadRequest)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "file is required", http.StatusBadRequest)
				return
			}
			defer file.Close()
			ver, _ := strconv.Atoi(r.FormValue("version"))
			m = Manifest{Name: header.Filename, Ver: ver}
		} else {
			if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
		}
		if m.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		state.mu.Lock()
		state.manifests = append(state.manifests, m)
		state.mu.Unlock()
		respondJSON(w, map[string]any{"ok": true, "manifest": m})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ===== External API integration =====
func fetchExternalDevices(base string) ([]Agent, error) {
	url := strings.TrimRight(base, "/") + "/admin/getAgents"
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/cbor")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d from external", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") == "application/cbor" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}

		// The API is expected to return a list of agents, e.g., [ [kid1, details1], ... ]
		var rawList []interface{}
		if err := cbor.Unmarshal(body, &rawList); err != nil {
			return nil, fmt.Errorf("failed to unmarshal CBOR device list: %w", err)
		}

		var agents []Agent
		for _, itemRaw := range rawList {
			item, ok := itemRaw.([]interface{})
			if !ok || len(item) < 2 {
				continue // Skip malformed items
			}

			var kid string
			switch v := item[0].(type) {
			case string:
				kid = v
			case []byte:
				kid = string(v)
			}
			detail, _ := item[1].(map[interface{}]interface{})

			agent := Agent{KID: kid}

			if attrsRaw, ok := detail["attributes"]; ok {
				if attrs, ok := attrsRaw.(map[interface{}]interface{}); ok {
					keys := []interface{}{256, int64(256), uint64(256)}
					for _, k := range keys {
						if v, found := attrs[k]; found {
							if b, ok := v.([]byte); ok {
								agent.Attributes.Ueid = hex.EncodeToString(b)
								break
							}
						}
					}
				}
			}
			agent.WappList = buildWappList(detail["wapp_list"])
			agents = append(agents, agent)
		}
		return agents, nil
	}

	// Fallback to JSON decoding for non-CBOR responses
	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return parseAgents(payload)
}

func fetchExternalManifests(base string) ([]Manifest, error) {
	url := strings.TrimRight(base, "/") + "/admin/getManifests"
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/cbor")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d from external", resp.StatusCode)
	}

	var manifests []Manifest
	if resp.Header.Get("Content-Type") == "application/cbor" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		jsonBytes, err := ConvertManifestsCBORToJSON(body)
		if err != nil {
			return nil, fmt.Errorf("failed to convert CBOR: %w", err)
		}
		if err := json.Unmarshal(jsonBytes, &manifests); err != nil {
			return nil, fmt.Errorf("failed to unmarshal converted JSON: %w", err)
		}
	} else {
		if err := json.NewDecoder(resp.Body).Decode(&manifests); err != nil {
			return nil, err
		}
	}
	return manifests, nil
}

func parseAgents(v any) ([]Agent, error) {
	var agents []Agent

	// If single object that looks like an Agent, try to decode it directly
	if m, ok := v.(map[string]any); ok {
		b, err := json.Marshal(m)
		if err == nil {
			var a Agent
			if err := json.Unmarshal(b, &a); err == nil {
				return []Agent{a}, nil
			}
		}
	}

	// If it's an array, handle multiple possible element formats
	if arr, ok := v.([]any); ok {
		for _, entry := range arr {
			// try unmarshalling as Agent
			if m, ok := entry.(map[string]any); ok {
				b, err := json.Marshal(m)
				if err == nil {
					var a Agent
					if err := json.Unmarshal(b, &a); err == nil {
						agents = append(agents, a)
						continue
					}
				}
			}

			// fallback: expect [kid, detail] pair
			pair, ok := entry.([]any)
			if !ok || len(pair) < 2 {
				continue
			}
			kid, _ := pair[0].(string)
			detail, _ := pair[1].(map[string]any)
			var deviceID string
			if attrs, ok := detail["attributes"].(map[string]any); ok {
				deviceID = extractDeviceID(attrs, kid)
			}
			if deviceID == "" {
				deviceID = kid
			}
			var ueid string
			if attrs, ok := detail["attributes"].(map[string]any); ok {
				if u, ok := attrs["ueid"].(string); ok {
					ueid = u
				} else {
					for _, val := range attrs {
						if s, ok := val.(string); ok {
							ueid = s
							break
						}
					}
				}
			}
			wapps := buildWappList(detail["wapp_list"])
			agents = append(agents, Agent{KID: deviceID, Attributes: Attribute{Ueid: ueid}, WappList: wapps})
		}
		return agents, nil
	}

	return nil, fmt.Errorf("unexpected payload format")
}

var reBuilding = regexp.MustCompile(`building-[a-zA-Z0-9_-]+`)

func extractDeviceID(attrs map[string]any, fallback string) string {
	if u, ok := attrs["ueid"]; ok {
		if s, ok := u.(string); ok {
			if m := reBuilding.FindString(s); m != "" {
				return m
			}
		}
	}
	for _, v := range attrs {
		if s, ok := v.(string); ok {
			if m := reBuilding.FindString(s); m != "" {
				return m
			}
		}
	}
	return fallback
}

func buildWappList(v any) []WappItem {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]WappItem, 0, len(list))
	for _, item := range list {
		name, ver := parseWappItem(item)
		if name == "" {
			continue
		}
		if ver < 0 {
			ver = 0
		}
		out = append(out, WappItem{Name: name, Ver: ver})
	}
	return out
}

func parseWappItem(item any) (name string, ver int) {
	ver = -1
	if m, ok := item.(map[string]any); ok {
		if v, ok := m["SUIT_Component_Identifier"]; ok {
			switch t := v.(type) {
			case []any:
				if len(t) > 0 {
					if s, ok := t[0].(string); ok {
						name = s
					} else {
						name = fmt.Sprint(t[0])
					}
				}
			case string:
				name = t
			default:
				name = fmt.Sprint(t)
			}
		}
		if v, ok := m["manifest-sequence-number"]; ok {
			if f, ok := v.(float64); ok {
				ver = int(math.Round(f))
			} else if i, ok := v.(int); ok {
				ver = i
			}
		}
		return
	}
	if a, ok := item.([]any); ok {
		if len(a) > 0 {
			if s, ok := a[0].(string); ok {
				name = s
			} else {
				name = fmt.Sprint(a[0])
			}
		}
		if len(a) > 1 {
			switch v := a[1].(type) {
			case float64:
				ver = int(math.Round(v))
			case int:
				ver = v
			case int64:
				ver = int(v)
			case uint64:
				ver = int(v)
			}
		}
		return
	}
	name = fmt.Sprint(item)
	return
}
