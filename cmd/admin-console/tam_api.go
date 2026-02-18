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
)

func fetchTAMDevices(base string) ([]Agent, error) {
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
		return nil, fmt.Errorf("status %d from TAM API", resp.StatusCode)
	}

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
		return decodeAgentsFromCBOR(body)
	}

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return parseAgents(payload)
}

func fetchTAMManifests(base string) ([]Manifest, error) {
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

	url := strings.TrimRight(base, "/") + "/tc-developer/addManifest"
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
