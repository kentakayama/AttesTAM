package main

import (
	"encoding/json"
	"fmt"
)

func decodeManifestsFromCBOR(body []byte) ([]TrustedComponent, error) {
	jsonBytes, err := ConvertManifestsCBORToJSON(body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert CBOR: %w", err)
	}
	var manifests []TrustedComponent
	if err := json.Unmarshal(jsonBytes, &manifests); err != nil {
		return nil, fmt.Errorf("failed to unmarshal converted JSON: %w", err)
	}
	return manifests, nil
}
