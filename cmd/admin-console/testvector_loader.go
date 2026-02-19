package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fxamacker/cbor/v2"
)

func loadTestVectorAgents() ([]Agent, error) {
	listBody, err := os.ReadFile(resolvePath(filepath.Join("testvector", "agentservice_listagents.cbor")))
	if err != nil {
		return nil, err
	}
	statusBody, err := os.ReadFile(resolvePath(filepath.Join("testvector", "agentservice_getagentstatus.cbor")))
	if err != nil {
		return nil, err
	}

	var listPayload any
	if err := cbor.Unmarshal(listBody, &listPayload); err != nil {
		return nil, fmt.Errorf("failed to decode ListAgents testvector: %w", err)
	}
	entries, ok := parseListAgentsPayload(listPayload)
	if !ok {
		return nil, fmt.Errorf("invalid ListAgents testvector format")
	}

	agents, err := decodeAgentsFromCBOR(statusBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GetAgentStatus testvector: %w", err)
	}
	lastUpdatedByKID := make(map[string]string, len(entries))
	for _, e := range entries {
		lastUpdatedByKID[string(e.AgentKID)] = formatUpdatedAt(e.UpdatedAt)
	}
	for i := range agents {
		agents[i].LastUpdate = lastUpdatedByKID[agents[i].KID]
	}
	return agents, nil
}

func loadTestVectorManifests() ([]TrustedComponent, error) {
	body, err := os.ReadFile(resolvePath(filepath.Join("testvector", "suitmanifestservice_listmanifests.cbor")))
	if err != nil {
		return nil, err
	}
	return decodeManifestsFromCBOR(body)
}
