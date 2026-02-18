package main

import (
	"encoding/json"
	"fmt"
)

func decodeManifestsFromCBOR(body []byte) ([]Manifest, error) {
	jsonBytes, err := ConvertManifestsCBORToJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert CBOR: %w", err)
	}
	var manifests []Manifest
	if err := json.Unmarshal(jsonBytes, &manifests); err != nil {
		return nil, fmt.Errorf("failed to unmarshal converted JSON: %w", err)
	}
	return manifests, nil
}
