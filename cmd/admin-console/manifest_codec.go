/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

func decodeManifestsFromCBOR(body []byte) ([]TrustedComponent, error) {
	var overviews []model.SuitManifestOverview
	if err := cbor.Unmarshal(body, &overviews); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	manifests := make([]TrustedComponent, 0, len(overviews))
	for _, overview := range overviews {
		name, ok := decodeCBORComponentID(overview.TrustedComponentID)
		if !ok {
			continue
		}
		manifests = append(manifests, TrustedComponent{
			Name:    name,
			Version: uint64(overview.SequenceNumber),
		})
	}
	return manifests, nil
}
