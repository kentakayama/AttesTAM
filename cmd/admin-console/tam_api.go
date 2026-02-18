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
)

func fetchTAMDevices(base string) ([]Agent, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	entries, err := fetchTAMListAgents(base, client)
	if err != nil {
		return nil, err
	}

	kids := make([]string, 0, len(entries))
	lastUpdatedByKID := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.KID == "" {
			continue
		}
		kids = append(kids, e.KID)
		lastUpdatedByKID[e.KID] = e.LastUpdated
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

type listAgentsEntry struct {
	KID         string
	LastUpdated string
}

func fetchTAMListAgents(base string, client *http.Client) ([]listAgentsEntry, error) {
	url := strings.TrimRight(base, "/") + "/AgentService/ListAgents"
	req, err := http.NewRequest(http.MethodPost, url, nil)
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

	var payload any
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		if err := cbor.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("failed to decode ListAgents cbor: %w", err)
		}
	} else {
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, err
		}
	}

	entries, ok := parseListAgentsPayload(payload)
	if !ok {
		return nil, fmt.Errorf("invalid ListAgents response format")
	}
	return entries, nil
}

func parseListAgentsPayload(payload any) ([]listAgentsEntry, bool) {
	arr, ok := payload.([]any)
	if !ok {
		return nil, false
	}
	out := make([]listAgentsEntry, 0, len(arr))
	for _, row := range arr {
		pair, ok := row.([]any)
		if !ok || len(pair) < 2 {
			continue
		}
		kid := toString(pair[0])
		if kid == "" {
			continue
		}
		out = append(out, listAgentsEntry{
			KID:         kid,
			LastUpdated: toString(pair[1]),
		})
	}
	return out, true
}

func fetchTAMGetAgentStatus(base string, client *http.Client, kids []string) ([]Agent, error) {
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

func fetchTAMManifests(base string) ([]Manifest, error) {
	url := strings.TrimRight(base, "/") + "/SUITManifestService/ListManifests"
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, nil)
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

	var manifests []Manifest
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
		"manifest":  Manifest{Name: header.Filename, Ver: ver},
		"tamStatus": resp.StatusCode,
		"tamBody":   strings.TrimSpace(string(respBody)),
	})
	return nil
}
