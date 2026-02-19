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
	"math"
	"regexp"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/kentakayama/tam-over-http/internal/tam"
)

func decodeAgentsFromCBOR(body []byte) ([]Agent, error) {
	var records []tam.AgentStatusRecord
	if err := cbor.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR device list: %w", err)
	}

	agents := make([]Agent, 0, len(records))
	for _, record := range records {
		agent := Agent{KID: string(record.AgentKID)}
		if len(record.Status.Attributes.DeviceUEID) > 0 {
			agent.Attributes.Ueid = hex.EncodeToString(record.Status.Attributes.DeviceUEID)
		}
		if len(record.Status.SuitManifests) > 0 {
			agent.InstalledTCList = make([]TrustedComponent, 0, len(record.Status.SuitManifests))
			for _, m := range record.Status.SuitManifests {
				name := toComponentID(m.TrustedComponentID)
				if len(name) == 0 {
					continue
				}
				agent.InstalledTCList = append(agent.InstalledTCList, TrustedComponent{
					Name:    name,
					Version: int(m.SequenceNumber),
				})
			}
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func parseAgents(v any) ([]Agent, error) {
	var agents []Agent

	if m, ok := v.(map[string]any); ok {
		b, err := json.Marshal(m)
		if err == nil {
			var a Agent
			if err := json.Unmarshal(b, &a); err == nil {
				return []Agent{a}, nil
			}
		}
		a := Agent{}
		if kid, ok := m["kid"].(string); ok {
			a.KID = kid
		}
		if attrs, ok := m["attribute"].(map[string]any); ok {
			if ueid, ok := attrs["ueid"].(string); ok {
				a.Attributes.Ueid = ueid
			}
		}
		if a.Attributes.Ueid == "" {
			if attrs, ok := m["attributes"].(map[string]any); ok {
				for _, val := range attrs {
					if s, ok := val.(string); ok {
						a.Attributes.Ueid = s
						break
					}
				}
			}
		}
		a.InstalledTCList = buildInstalledTCList(extractInstalledTCListFromJSONDetail(m))
		return []Agent{a}, nil
	}

	if arr, ok := v.([]any); ok {
		for _, entry := range arr {
			if m, ok := entry.(map[string]any); ok {
				b, err := json.Marshal(m)
				if err == nil {
					var a Agent
					if err := json.Unmarshal(b, &a); err == nil {
						agents = append(agents, a)
						continue
					}
				}
			}

			pair, ok := entry.([]any)
			if !ok || len(pair) < 2 {
				continue
			}
			kid, _ := pair[0].(string)
			detail, _ := pair[1].(map[string]any)
			var deviceID string
			if attrs, ok := detail["attributes"].(map[string]any); ok {
				deviceID = extractDeviceID(attrs, kid)
			}
			if deviceID == "" {
				deviceID = kid
			}
			var ueid string
			if attrs, ok := detail["attributes"].(map[string]any); ok {
				if u, ok := attrs["ueid"].(string); ok {
					ueid = u
				} else {
					for _, val := range attrs {
						if s, ok := val.(string); ok {
							ueid = s
							break
						}
					}
				}
			}
			installedTCs := buildInstalledTCList(extractInstalledTCListFromJSONDetail(detail))
			agents = append(agents, Agent{KID: deviceID, Attributes: Attribute{Ueid: ueid}, InstalledTCList: installedTCs})
		}
		return agents, nil
	}

	return nil, fmt.Errorf("unexpected payload format")
}

var reBuilding = regexp.MustCompile(`building-[a-zA-Z0-9_-]+`)

func extractDeviceID(attrs map[string]any, fallback string) string {
	if u, ok := attrs["ueid"]; ok {
		if s, ok := u.(string); ok {
			if m := reBuilding.FindString(s); m != "" {
				return m
			}
		}
	}
	for _, v := range attrs {
		if s, ok := v.(string); ok {
			if m := reBuilding.FindString(s); m != "" {
				return m
			}
		}
	}
	return fallback
}

func buildInstalledTCList(v any) []TrustedComponent {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]TrustedComponent, 0, len(list))
	for _, item := range list {
		name, ver := parseInstalledTCItem(item)
		if len(name) == 0 {
			continue
		}
		if ver < 0 {
			ver = 0
		}
		out = append(out, TrustedComponent{Name: name, Version: ver})
	}
	return out
}

func parseInstalledTCItem(item any) (name suit.ComponentID, ver int) {
	ver = -1
	if m, ok := item.(map[string]any); ok {
		if v, ok := m["SUIT_Component_Identifier"]; ok {
			name = toComponentID(v)
		}
		if v, ok := m["manifest-sequence-number"]; ok {
			if f, ok := v.(float64); ok {
				ver = int(math.Round(f))
			} else if i, ok := v.(int); ok {
				ver = i
			}
		}
		return
	}
	if a, ok := item.([]any); ok {
		if len(a) > 0 {
			name = toComponentID(a[0])
		}
		if len(a) > 1 {
			switch v := a[1].(type) {
			case float64:
				ver = int(math.Round(v))
			case int:
				ver = v
			case int64:
				ver = int(v)
			case uint64:
				ver = int(v)
			}
		}
		return
	}
	name = toComponentID(item)
	return
}

func extractInstalledTCListFromJSONDetail(detail map[string]any) any {
	if v, ok := detail["installed-tc"]; ok {
		return v
	}
	return nil
}

func toComponentID(v any) suit.ComponentID {
	switch t := v.(type) {
	case nil:
		return nil
	case suit.ComponentID:
		if len(t) == 0 {
			return nil
		}
		return t
	case []byte:
		if id, ok := decodeCBORComponentID(t); ok {
			return id
		}
		return suit.ComponentID{suit.ComponentIDBytes(t)}
	case string:
		if t == "" {
			return nil
		}
		return suit.ComponentID{suit.ComponentIDBytes([]byte(t))}
	case []any:
		out := make(suit.ComponentID, 0, len(t))
		for _, p := range t {
			switch b := p.(type) {
			case []byte:
				out = append(out, suit.ComponentIDBytes(b))
			case string:
				out = append(out, suit.ComponentIDBytes([]byte(b)))
			default:
				s := fmt.Sprint(b)
				if s != "" {
					out = append(out, suit.ComponentIDBytes([]byte(s)))
				}
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		s := fmt.Sprint(t)
		if s == "" {
			return nil
		}
		return suit.ComponentID{suit.ComponentIDBytes([]byte(s))}
	}
}

func decodeCBORComponentID(raw []byte) (suit.ComponentID, bool) {
	var id suit.ComponentID
	if err := cbor.Unmarshal(raw, &id); err != nil || len(id) == 0 {
		return nil, false
	}
	return id, true
}
