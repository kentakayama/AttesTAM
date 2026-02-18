package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// ConvertCBORToJSON converts the specific CBOR structure to the target JSON format.
// Input CBOR structure: [kid(string), { "attributes": { 256: bytes }, "installed-tc": [ [name, ver], ... ] }]
func ConvertCBORToJSON(data []byte) ([]byte, error) {
	// 1. Unmarshal CBOR into a generic slice since the root is an array
	var raw []interface{}
	if err := cbor.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	if len(raw) < 2 {
		return nil, fmt.Errorf("invalid CBOR structure: expected at least 2 elements")
	}

	// 2. Extract KID
	kid, ok := raw[0].(string)
	if !ok {
		return nil, fmt.Errorf("expected string at index 0")
	}

	// 3. Extract details map
	details, ok := raw[1].(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map at index 1")
	}

	// 4. Build Target Structure
	target := Agent{
		KID: kid,
	}

	// Process Attributes (Key 256 -> UEID)
	if attrsRaw, ok := details["attributes"]; ok {
		if attrs, ok := attrsRaw.(map[interface{}]interface{}); ok {
			// Check for key 256 (handled as int, int64, or uint64 depending on decoder)
			var ueidBytes []byte
			keys := []interface{}{256, int64(256), uint64(256)}
			for _, k := range keys {
				if v, found := attrs[k]; found {
					if b, ok := v.([]byte); ok {
						ueidBytes = b
						break
					}
				}
			}
			if ueidBytes != nil {
				target.Attributes.Ueid = hex.EncodeToString(ueidBytes)
			}
		}
	}

	// Process Installed TC List
	tcsRaw := details["installed-tc"]
	if tcsRaw != nil {
		if tcs, ok := tcsRaw.([]interface{}); ok {
			for _, item := range tcs {
				if pair, ok := item.([]interface{}); ok && len(pair) >= 2 {
					name := toComponentID(pair[0])
					var ver int
					switch v := pair[1].(type) {
					case int:
						ver = v
					case int64:
						ver = int(v)
					case uint64:
						ver = int(v)
					case float64:
						ver = int(v)
					}
					if len(name) > 0 {
						target.InstalledTCList = append(target.InstalledTCList, InstalledTCItem{Name: name, Ver: ver})
					}
				}
			}
		}
	}

	// 5. Marshal to JSON
	return json.MarshalIndent(target, "", "  ")
}

// ConvertManifestsCBORToJSON converts the manifest list CBOR structure to JSON.
// Input CBOR structure: [ [ scid_bytes, version_int ], ... ]
// Where scid_bytes is CBOR encoded [ bstr(name) ]
func ConvertManifestsCBORToJSON(data []byte) ([]byte, error) {
	var raw []interface{}
	if err := cbor.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CBOR: %w", err)
	}

	var result []Manifest

	for _, item := range raw {
		arr, ok := item.([]interface{})
		if !ok || len(arr) < 2 {
			continue
		}

		// Decode Version
		var ver int
		switch v := arr[1].(type) {
		case int:
			ver = v
		case int64:
			ver = int(v)
		case uint64:
			ver = int(v)
		case float64:
			ver = int(v)
		}

		// Decode Name from SCID
		var name string
		if scidBytes, ok := arr[0].([]byte); ok {
			var scidParts []interface{}
			if err := cbor.Unmarshal(scidBytes, &scidParts); err == nil && len(scidParts) > 0 {
				if n, ok := scidParts[0].([]byte); ok {
					name = string(n)
				}
			}
		}

		result = append(result, Manifest{
			Name: name,
			Ver:  ver,
		})
	}

	return json.MarshalIndent(result, "", "  ")
}
