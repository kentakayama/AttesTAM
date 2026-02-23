/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

func TestDecodeAgentsFromCBOR(t *testing.T) {
	raw := []tam.AgentStatusRecord{
		{
			AgentKID: []byte("kid-1"),
			Status: tam.AgentStatus{
				Attributes: tam.AgentAttributes{
					DeviceUEID: []byte{0x01, 0x02, 0x03},
				},
				SuitManifests: []model.SuitManifestOverview{
					{
						TrustedComponentID: mustMarshalCBOR(t, []any{[]byte("app-a")}),
						SequenceNumber:     2,
					},
				},
			},
		},
	}
	body, err := cbor.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal cbor: %v", err)
	}

	agents, err := decodeAgentsFromCBOR(body)
	if err != nil {
		t.Fatalf("decodeAgentsFromCBOR: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if !bytes.Equal(agents[0].KID, []byte("kid-1")) {
		t.Fatalf("unexpected kid: %q", string(agents[0].KID))
	}
	if !bytes.Equal([]byte(agents[0].Attributes.Ueid), []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("unexpected ueid: %x", []byte(agents[0].Attributes.Ueid))
	}
	if len(agents[0].InstalledTCList) != 1 || agents[0].InstalledTCList[0].Version != 2 {
		t.Fatalf("unexpected installed tc list: %+v", agents[0].InstalledTCList)
	}
	if len(agents[0].InstalledTCList[0].Name) != 1 || string(agents[0].InstalledTCList[0].Name[0]) != "app-a" {
		t.Fatalf("unexpected installed tc list: %+v", agents[0].InstalledTCList)
	}
}

