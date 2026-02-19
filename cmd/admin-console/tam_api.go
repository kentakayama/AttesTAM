/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

type cachedAgent struct {
	agent      Agent
	lastUpdate time.Time
}

type tamDevicesCache struct {
	mu    sync.RWMutex
	byKID map[string]cachedAgent
}

var tamDeviceCaches = struct {
	mu     sync.Mutex
	byBase map[string]*tamDevicesCache
}{
	byBase: make(map[string]*tamDevicesCache),
}

func getTAMDevicesCache(base string) *tamDevicesCache {
	tamDeviceCaches.mu.Lock()
	defer tamDeviceCaches.mu.Unlock()

	cache, ok := tamDeviceCaches.byBase[base]
	if ok {
		return cache
	}
	cache = &tamDevicesCache{
		byKID: make(map[string]cachedAgent),
	}
	tamDeviceCaches.byBase[base] = cache
	return cache
}

func resetTAMDeviceCachesForTest() {
	tamDeviceCaches.mu.Lock()
	defer tamDeviceCaches.mu.Unlock()
	tamDeviceCaches.byBase = make(map[string]*tamDevicesCache)
}

func fetchTAMDevices(base string) ([]Agent, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	keys, err := fetchTAMListAgents(base, client)
	if err != nil {
		return nil, err
	}

	lastUpdatedByKID := make(map[string]time.Time, len(keys))
	orderedKIDs := make([]string, 0, len(keys))
	cache := getTAMDevicesCache(base)

	cache.mu.RLock()
	cachedSnapshot := make(map[string]cachedAgent, len(cache.byKID))
	for kid, entry := range cache.byKID {
		cachedSnapshot[kid] = entry
	}
	cache.mu.RUnlock()

	kidsToFetch := make([][]byte, 0, len(keys))
	for _, key := range keys {
		if len(key.AgentKID) == 0 {
			continue
		}
		kid := string(key.AgentKID)
		orderedKIDs = append(orderedKIDs, kid)
		lastUpdated := key.UpdatedAt
		lastUpdatedByKID[kid] = lastUpdated
		cached, ok := cachedSnapshot[kid]
		if !ok || !cached.lastUpdate.Equal(lastUpdated) {
			kidsToFetch = append(kidsToFetch, key.AgentKID)
		}
	}
	if len(orderedKIDs) == 0 {
		cache.mu.Lock()
		cache.byKID = make(map[string]cachedAgent)
		cache.mu.Unlock()
		return []Agent{}, nil
	}

	if len(kidsToFetch) > 0 {
		agents, err := fetchTAMGetAgentStatus(base, client, kidsToFetch)
		if err != nil {
			return nil, err
		}
		for i := range agents {
			agents[i].LastUpdate = lastUpdatedByKID[string(agents[i].KID)]
		}

		cache.mu.Lock()
		for _, agent := range agents {
			cache.byKID[string(agent.KID)] = cachedAgent{
				agent:      agent,
				lastUpdate: lastUpdatedByKID[string(agent.KID)],
			}
		}
		for kid := range cache.byKID {
			if _, ok := lastUpdatedByKID[kid]; !ok {
				delete(cache.byKID, kid)
			}
		}
		cache.mu.Unlock()
	} else {
		cache.mu.Lock()
		for kid := range cache.byKID {
			if _, ok := lastUpdatedByKID[kid]; !ok {
				delete(cache.byKID, kid)
			}
		}
		cache.mu.Unlock()
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	agents := make([]Agent, 0, len(orderedKIDs))
	for _, kid := range orderedKIDs {
		if entry, ok := cache.byKID[kid]; ok {
			agents = append(agents, entry.agent)
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
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		return nil, fmt.Errorf("unsupported response Content-Type %q from TAM API", resp.Header.Get("Content-Type"))
	}
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
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		return nil, fmt.Errorf("unsupported response Content-Type %q from TAM API", resp.Header.Get("Content-Type"))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	return decodeAgentsFromCBOR(raw)
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

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/cbor") {
		return nil, fmt.Errorf("unsupported response Content-Type %q from TAM API", resp.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	manifests, err := decodeManifestsFromCBOR(body)
	if err != nil {
		return nil, err
	}
	return manifests, nil
}

func postTAMManifest(w http.ResponseWriter, r *http.Request, base string) error {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return fmt.Errorf("failed to parse form: %w", err)
	}
	file, _, err := r.FormFile("file")
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
	req.Header.Set("Content-Type", "application/suit-envelope+cose")

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

	respondJSON(w, map[string]any{
		"ok": true,
	})
	return nil
}
