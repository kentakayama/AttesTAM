package main

import (
	"os"
	"path/filepath"
)

func loadTestVectorDevices() ([]Agent, error) {
	body, err := os.ReadFile(resolvePath(filepath.Join("testvector", "devices.cbor")))
	if err != nil {
		return nil, err
	}
	return decodeAgentsFromCBOR(body)
}

func loadTestVectorManifests() ([]Manifest, error) {
	body, err := os.ReadFile(resolvePath(filepath.Join("testvector", "manifests.cbor")))
	if err != nil {
		return nil, err
	}
	return decodeManifestsFromCBOR(body)
}
