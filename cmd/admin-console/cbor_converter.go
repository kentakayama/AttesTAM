/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

// ConvertCBORToJSON converts a TAM AgentStatusRecord CBOR payload to the target JSON format.
func ConvertCBORToJSON(data []byte) ([]byte, error) {
	var record tam.AgentStatusRecord
	if err := cbor.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	target := Agent{
		KID: string(record.AgentKID),
	}
	if len(record.Status.Attributes.DeviceUEID) > 0 {
		target.Attributes.Ueid = hex.EncodeToString(record.Status.Attributes.DeviceUEID)
	}
	if len(record.Status.SuitManifests) > 0 {
		target.InstalledTCList = make([]TrustedComponent, 0, len(record.Status.SuitManifests))
		for _, m := range record.Status.SuitManifests {
			name := toComponentID(m.TrustedComponentID)
			if len(name) == 0 {
				continue
			}
			target.InstalledTCList = append(target.InstalledTCList, TrustedComponent{
				Name:    name,
				Version: int(m.SequenceNumber),
			})
		}
	}
	return json.MarshalIndent(target, "", "  ")
}

// ConvertManifestsCBORToJSON converts TAM manifest overview CBOR list to JSON.
func ConvertManifestsCBORToJSON(data []byte) ([]byte, error) {
	var overviews []model.SuitManifestOverview
	if err := cbor.Unmarshal(data, &overviews); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	result := make([]TrustedComponent, 0, len(overviews))
	for _, overview := range overviews {
		name := toComponentID(overview.TrustedComponentID)
		if len(name) == 0 {
			continue
		}
		result = append(result, TrustedComponent{
			Name:    name,
			Version: int(overview.SequenceNumber),
		})
	}
	return json.MarshalIndent(result, "", "  ")
}
