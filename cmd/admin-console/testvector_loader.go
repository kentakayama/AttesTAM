/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/tam"
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

	var entries []tam.AgentStatusKey
	if err := cbor.Unmarshal(listBody, &entries); err != nil {
		return nil, fmt.Errorf("failed to decode ListAgents testvector: %w", err)
	}

	agents, err := decodeAgentsFromCBOR(statusBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GetAgentStatus testvector: %w", err)
	}
	lastUpdatedByKID := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		lastUpdatedByKID[string(e.AgentKID)] = e.UpdatedAt
	}
	for i := range agents {
		agents[i].LastUpdate = lastUpdatedByKID[string(agents[i].KID)]
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
