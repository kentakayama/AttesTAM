package main

import (
	"encoding/json"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestDecodeAgentsFromCBOR(t *testing.T) {
	raw := []any{
		[]any{
			"kid-1",
			map[any]any{
				"attributes": map[any]any{
					256: []byte{0x01, 0x02, 0x03},
				},
				"wapp_list": []any{
					[]any{"app-a", uint64(2)},
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
	if len(agents[0].WappList) != 1 || agents[0].WappList[0].Name != "app-a" || agents[0].WappList[0].Ver != 2 {
		t.Fatalf("unexpected wapp list: %+v", agents[0].WappList)
	}
}

func TestParseAgentsSingleObject(t *testing.T) {
	payload := map[string]any{
		"kid":       "dev-1",
		"attribute": map[string]any{"ueid": "ueid-1"},
		"wapp_list": []any{map[string]any{"name": "app", "ver": 1}},
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
				"wapp_list": []any{
					[]any{"wapp-1", float64(3.2)},
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
	if len(agents[0].WappList) != 1 || agents[0].WappList[0].Ver != 3 {
		t.Fatalf("unexpected wapp list: %+v", agents[0].WappList)
	}
}

func TestParseAgentsUnexpectedPayload(t *testing.T) {
	_, err := parseAgents("invalid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConvertCBORToJSON(t *testing.T) {
	raw := []any{
		"kid-x",
		map[any]any{
			"attributes": map[any]any{
				int64(256): []byte{0xaa, 0xbb},
			},
			"wapp_list": []any{
				[]any{"wapp-x", int64(5)},
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
}
