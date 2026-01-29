/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import (
	"time"

	"github.com/fxamacker/cbor/v2"
)

// AgentStatus represents an agent's attributes such as UEID and possession of a SUIT manifest.
type AgentStatus struct {
	AgentKID      []byte
	DeviceUEID    []byte
	SuitManifests []SuitManifestOverview
	UpdatedAt     time.Time
}

func (s *AgentStatus) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal([]any{s.AgentKID, map[string]any{
		"attributes": map[int]any{
			/* ueid */ 256: s.DeviceUEID,
		},
		"wapp_list": s.SuitManifests,
	}})
}
