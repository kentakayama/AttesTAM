/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestDecodeManifestsFromCBOR(t *testing.T) {
	scid, err := cbor.Marshal([]any{[]byte("manifest-a")})
	if err != nil {
		t.Fatalf("marshal scid: %v", err)
	}
	raw := []any{
		[]any{scid, uint64(7)},
	}
	body, err := cbor.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal manifests cbor: %v", err)
	}

	manifests, err := decodeManifestsFromCBOR(body)
	if err != nil {
		t.Fatalf("decodeManifestsFromCBOR: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if componentIDDisplayName(manifests[0].Name) != "['manifest-a']" || manifests[0].Version != 7 {
		t.Fatalf("unexpected manifest: %+v", manifests[0])
	}
}

func TestDecodeManifestsFromCBORInvalid(t *testing.T) {
	_, err := decodeManifestsFromCBOR([]byte{0xff})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
