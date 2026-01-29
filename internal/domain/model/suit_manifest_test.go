/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuitManifestOverview_Marshal_OK(t *testing.T) {
	expected := []byte{
		0x82, 0x4B, 0x81, 0x49, 0x61, 0x70, 0x70, 0x31, 0x2E, 0x77, 0x61, 0x73, 0x6D, 0x02,
	}
	m := SuitManifestOverview{
		TrustedComponentID: []byte{0x81, 0x49, 0x61, 0x70, 0x70, 0x31, 0x2E, 0x77, 0x61, 0x73, 0x6D}, // ['app1.wasm']
		SequenceNumber:     2,
	}

	encoded, err := cbor.Marshal(m)
	require.Nil(t, err)
	assert.Equal(t, expected, encoded)
}
