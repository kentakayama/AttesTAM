package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"

	"github.com/fxamacker/cbor/v2"
)

func decodeAgentsFromCBOR(body []byte) ([]Agent, error) {
	var rawList []interface{}
	if err := cbor.Unmarshal(body, &rawList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR device list: %w", err)
	}

	var agents []Agent
	for _, itemRaw := range rawList {
		item, ok := itemRaw.([]interface{})
		if !ok || len(item) < 2 {
			continue
		}

		var kid string
		switch v := item[0].(type) {
		case string:
			kid = v
		case []byte:
			kid = string(v)
		}
		detail, _ := item[1].(map[interface{}]interface{})

		agent := Agent{KID: kid}
		if attrsRaw, ok := detail["attributes"]; ok {
			if attrs, ok := attrsRaw.(map[interface{}]interface{}); ok {
				keys := []interface{}{256, int64(256), uint64(256)}
				for _, k := range keys {
					if v, found := attrs[k]; found {
						if b, ok := v.([]byte); ok {
							agent.Attributes.Ueid = hex.EncodeToString(b)
							break
						}
					}
				}
			}
		}
		agent.WappList = buildWappList(detail["wapp_list"])
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
			wapps := buildWappList(detail["wapp_list"])
			agents = append(agents, Agent{KID: deviceID, Attributes: Attribute{Ueid: ueid}, WappList: wapps})
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

func buildWappList(v any) []WappItem {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]WappItem, 0, len(list))
	for _, item := range list {
		name, ver := parseWappItem(item)
		if name == "" {
			continue
		}
		if ver < 0 {
			ver = 0
		}
		out = append(out, WappItem{Name: name, Ver: ver})
	}
	return out
}

func parseWappItem(item any) (name string, ver int) {
	ver = -1
	if m, ok := item.(map[string]any); ok {
		if v, ok := m["SUIT_Component_Identifier"]; ok {
			switch t := v.(type) {
			case []any:
				if len(t) > 0 {
					if s, ok := t[0].(string); ok {
						name = s
					} else {
						name = fmt.Sprint(t[0])
					}
				}
			case string:
				name = t
			default:
				name = fmt.Sprint(t)
			}
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
			if s, ok := a[0].(string); ok {
				name = s
			} else {
				name = fmt.Sprint(a[0])
			}
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
	name = fmt.Sprint(item)
	return
}
