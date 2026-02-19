/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"encoding/json"
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

func TestConvertCBORToJSON(t *testing.T) {
	raw := tam.AgentStatusRecord{
		AgentKID: []byte("kid-x"),
		Status: tam.AgentStatus{
			Attributes: tam.AgentAttributes{
				DeviceUEID: []byte{0xaa, 0xbb},
			},
			SuitManifests: []model.SuitManifestOverview{
				{
					TrustedComponentID: mustMarshalCBOR(t, []any{[]byte("tc-x")}),
					SequenceNumber:     5,
				},
			},
		},
	}
	body, err := cbor.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal cbor: %v", err)
	}

	j, err := ConvertCBORToJSON(body)
	if err != nil {
		t.Fatalf("ConvertCBORToJSON: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(j, &m); err != nil {
		t.Fatalf("unmarshal json map: %v", err)
	}
	if kid, _ := m["kid"].(string); kid != "kid-x" {
		t.Fatalf("unexpected kid in json: %#v", m["kid"])
	}
	attr, _ := m["attribute"].(map[string]any)
	if ueid, _ := attr["ueid"].(string); ueid != "aabb" {
		t.Fatalf("unexpected ueid in json: %#v", attr["ueid"])
	}
	wlist, _ := m["installed-tc"].([]any)
	if len(wlist) != 1 {
		t.Fatalf("unexpected installed tc list in json: %#v", m["installed-tc"])
	}
	w0, _ := wlist[0].(map[string]any)
	if gotName, _ := w0["name"].(string); gotName != "['tc-x']" {
		t.Fatalf("unexpected installed tc name in json: %#v", w0["name"])
	}
}
