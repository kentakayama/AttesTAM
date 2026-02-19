/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/tam"
	"github.com/veraison/eat"
)

// ConvertCBORToJSON converts a TAM AgentStatusRecord CBOR payload to the target JSON format.
func ConvertCBORToJSON(data []byte) ([]byte, error) {
	var record tam.AgentStatusRecord
	if err := cbor.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	target := Agent{
		KID: record.AgentKID,
	}
	if len(record.Status.Attributes.DeviceUEID) > 0 {
		target.Attributes.Ueid = eat.UEID(record.Status.Attributes.DeviceUEID)
	}
	if len(record.Status.SuitManifests) > 0 {
		target.InstalledTCList = make([]TrustedComponent, 0, len(record.Status.SuitManifests))
		for _, m := range record.Status.SuitManifests {
			name, ok := decodeCBORComponentID(m.TrustedComponentID)
			if !ok {
				continue
			}
			target.InstalledTCList = append(target.InstalledTCList, TrustedComponent{
				Name:    name,
				Version: uint64(m.SequenceNumber),
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
		name, ok := decodeCBORComponentID(overview.TrustedComponentID)
		if !ok {
			continue
		}
		result = append(result, TrustedComponent{
			Name:    name,
			Version: uint64(overview.SequenceNumber),
		})
	}
	return json.MarshalIndent(result, "", "  ")
}
