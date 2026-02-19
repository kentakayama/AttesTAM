package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

func fetchTAMDevices(base string) ([]Agent, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	keys, err := fetchTAMListAgents(base, client)
	if err != nil {
		return nil, err
	}

	kids := make([][]byte, 0, len(keys))
	lastUpdatedByKID := make(map[string]string, len(keys))
	for _, key := range keys {
		if len(key.AgentKID) == 0 {
			continue
		}
		kid := string(key.AgentKID)
		kids = append(kids, key.AgentKID)
		lastUpdatedByKID[kid] = formatUpdatedAt(key.UpdatedAt)
	}
	if len(kids) == 0 {
		return []Agent{}, nil
	}

	agents, err := fetchTAMGetAgentStatus(base, client, kids)
	if err != nil {
		return nil, err
	}
	for i := range agents {
		if lu, ok := lastUpdatedByKID[agents[i].KID]; ok {
			agents[i].LastUpdate = lu
		}
	}
	return agents, nil
}

func fetchTAMListAgents(base string, client *http.Client) ([]tam.AgentStatusKey, error) {
	url := strings.TrimRight(base, "/") + "/AgentService/ListAgents"
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
		return nil, fmt.Errorf("status %d from TAM API", resp.StatusCode)
	}

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		var keys []tam.AgentStatusKey
		if err := cbor.Unmarshal(body, &keys); err != nil {
			return nil, fmt.Errorf("failed to decode ListAgents cbor: %w", err)
		}
		return keys, nil
	}

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	keys, ok := parseListAgentsPayload(payload)
	if !ok {
		return nil, fmt.Errorf("invalid ListAgents response format")
	}
	return keys, nil
}

func parseListAgentsPayload(payload any) ([]tam.AgentStatusKey, bool) {
	arr, ok := payload.([]any)
	if !ok {
		return nil, false
	}
	out := make([]tam.AgentStatusKey, 0, len(arr))
	for _, row := range arr {
		pair, ok := row.([]any)
		if !ok || len(pair) < 2 {
			continue
		}
		kid := toString(pair[0])
		if kid == "" {
			continue
		}
		out = append(out, tam.AgentStatusKey{
			AgentKID:  []byte(kid),
			UpdatedAt: parseUpdatedAt(pair[1]),
		})
	}
	return out, true
}

func fetchTAMGetAgentStatus(base string, client *http.Client, kids [][]byte) ([]Agent, error) {
	url := strings.TrimRight(base, "/") + "/AgentService/GetAgentStatus"
	body, err := cbor.Marshal(kids)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GetAgentStatus request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d from TAM API", resp.StatusCode)
	}

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		return decodeAgentsFromCBOR(raw)
	}

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return parseAgents(payload)
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func parseUpdatedAt(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		tm, err := time.Parse(time.RFC3339, t)
		if err == nil {
			return tm
		}
	case int64:
		return time.Unix(t, 0).UTC()
	case uint64:
		return time.Unix(int64(t), 0).UTC()
	case int:
		return time.Unix(int64(t), 0).UTC()
	}
	return time.Time{}
}

func formatUpdatedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func fetchTAMManifests(base string) ([]TrustedComponent, error) {
	url := strings.TrimRight(base, "/") + "/SUITManifestService/ListManifests"
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
		return nil, fmt.Errorf("status %d from TAM API", resp.StatusCode)
	}

	var manifests []TrustedComponent
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		manifests, err = decodeManifestsFromCBOR(body)
		if err != nil {
			return nil, err
		}
	} else {
		if err := json.NewDecoder(resp.Body).Decode(&manifests); err != nil {
			return nil, err
		}
	}
	return manifests, nil
}

func postTAMManifest(w http.ResponseWriter, r *http.Request, base string) error {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return fmt.Errorf("failed to parse form: %w", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("file is required")
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read upload file: %w", err)
	}

	url := strings.TrimRight(base, "/") + "/SUITManifestService/RegisterManifest"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/suit-envelope+cose"
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read TAM API response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d from TAM API: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	ver, _ := strconv.Atoi(r.FormValue("version"))
	respondJSON(w, map[string]any{
		"ok":        true,
		"manifest":  TrustedComponent{Name: toComponentID(header.Filename), Version: ver},
		"tamStatus": resp.StatusCode,
		"tamBody":   strings.TrimSpace(string(respBody)),
	})
	return nil
}
