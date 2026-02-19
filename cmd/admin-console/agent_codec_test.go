/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
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
	if agents[0].KID != "kid-1" {
		t.Fatalf("unexpected kid: %q", agents[0].KID)
	}
	if agents[0].Attributes.Ueid != "010203" {
		t.Fatalf("unexpected ueid: %q", agents[0].Attributes.Ueid)
	}
	if len(agents[0].InstalledTCList) != 1 || agents[0].InstalledTCList[0].Version != 2 {
		t.Fatalf("unexpected installed tc list: %+v", agents[0].InstalledTCList)
	}
	if len(agents[0].InstalledTCList[0].Name) != 1 || string(agents[0].InstalledTCList[0].Name[0]) != "app-a" {
		t.Fatalf("unexpected installed tc list: %+v", agents[0].InstalledTCList)
	}
}

func TestParseAgentsSingleObject(t *testing.T) {
	payload := map[string]any{
		"kid":          "dev-1",
		"attribute":    map[string]any{"ueid": "ueid-1"},
		"installed-tc": []any{map[string]any{"name": []string{"YXBw"}, "version": 1}},
	}
	agents, err := parseAgents(payload)
	if err != nil {
		t.Fatalf("parseAgents: %v", err)
	}
	if len(agents) != 1 || agents[0].KID != "dev-1" || agents[0].Attributes.Ueid != "ueid-1" {
		t.Fatalf("unexpected agents: %+v", agents)
	}
}

func TestParseAgentsPairFormat(t *testing.T) {
	payload := []any{
		[]any{
			"fallback-kid",
			map[string]any{
				"attributes": map[string]any{
					"ueid": "urn:example:building-alpha",
				},
				"installed-tc": []any{
					[]any{"tc-1", float64(3.2)},
				},
			},
		},
	}
	agents, err := parseAgents(payload)
	if err != nil {
		t.Fatalf("parseAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].KID != "building-alpha" {
		t.Fatalf("unexpected extracted kid: %q", agents[0].KID)
	}
	if len(agents[0].InstalledTCList) != 1 || agents[0].InstalledTCList[0].Version != 3 {
		t.Fatalf("unexpected installed tc list: %+v", agents[0].InstalledTCList)
	}
}

func TestParseAgentsUnexpectedPayload(t *testing.T) {
	_, err := parseAgents("invalid")
	if err == nil {
		t.Fatal("expected error, got nil")
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

	var got Agent
	if err := json.Unmarshal(j, &got); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if got.KID != "kid-x" || got.Attributes.Ueid != "aabb" {
		t.Fatalf("unexpected converted agent: %+v", got)
	}
	var m map[string]any
	if err := json.Unmarshal(j, &m); err != nil {
		t.Fatalf("unmarshal json map: %v", err)
	}
	wlist, _ := m["installed-tc"].([]any)
	if len(wlist) != 1 {
		t.Fatalf("unexpected installed tc list in json: %#v", m["installed-tc"])
	}
	w0, _ := wlist[0].(map[string]any)
	if gotName, _ := w0["name"].(string); gotName != "tc-x" {
		t.Fatalf("unexpected installed tc name in json: %#v", w0["name"])
	}
}
