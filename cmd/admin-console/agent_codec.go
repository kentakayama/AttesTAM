/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/kentakayama/tam-over-http/internal/tam"
	"github.com/veraison/eat"
)

func decodeAgentsFromCBOR(body []byte) ([]Agent, error) {
	var records []tam.AgentStatusRecord
	if err := cbor.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR device list: %w", err)
	}

	agents := make([]Agent, 0, len(records))
	for _, record := range records {
		agent := Agent{KID: record.AgentKID}
		if len(record.Status.Attributes.DeviceUEID) > 0 {
			agent.Attributes.Ueid = eat.UEID(record.Status.Attributes.DeviceUEID)
		}
		if len(record.Status.SuitManifests) > 0 {
			agent.InstalledTCList = make([]TrustedComponent, 0, len(record.Status.SuitManifests))
			for _, m := range record.Status.SuitManifests {
				name, ok := decodeCBORComponentID(m.TrustedComponentID)
				if !ok {
					continue
				}
				agent.InstalledTCList = append(agent.InstalledTCList, TrustedComponent{
					Name:    name,
					Version: uint64(m.SequenceNumber),
				})
			}
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func componentIDFromFilename(filename string) suit.ComponentID {
	if filename == "" {
		return nil
	}
	return suit.ComponentID{suit.ComponentIDBytes([]byte(filename))}
}

func decodeCBORComponentID(raw []byte) (suit.ComponentID, bool) {
	var id suit.ComponentID
	if err := cbor.Unmarshal(raw, &id); err != nil || len(id) == 0 {
		return nil, false
	}
	return id, true
}
