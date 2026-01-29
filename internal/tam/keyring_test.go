/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"crypto"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/veraison/go-cose"
)

var (
	x = []byte{
		0x65, 0xed, 0xa5, 0xa1, 0x25, 0x77, 0xc2, 0xba, 0xe8, 0x29, 0x43, 0x7f, 0xe3, 0x38, 0x70, 0x1a,
		0x10, 0xaa, 0xa3, 0x75, 0xe1, 0xbb, 0x5b, 0x5d, 0xe1, 0x08, 0xde, 0x43, 0x9c, 0x08, 0x55, 0x1d,
	}
	y = []byte{
		0x1e, 0x52, 0xed, 0x75, 0x70, 0x11, 0x63, 0xf7, 0xf9, 0xe4, 0x0d, 0xdf, 0x9f, 0x34, 0x1b, 0x3d,
		0xc9, 0xba, 0x86, 0x0a, 0xf7, 0xe0, 0xca, 0x7c, 0xa7, 0xe9, 0xee, 0xcd, 0x00, 0x84, 0xd1, 0x9c,
	}
	key = cose.Key{
		Type:      cose.KeyTypeEC2,
		Algorithm: cose.AlgorithmESP256,
		Params: map[any]any{
			cose.KeyLabelEC2Curve: cose.CurveP256,
			cose.KeyLabelEC2X:     x,
			cose.KeyLabelEC2Y:     y,
		},
	}
)

func TestKeyring_GetSet(t *testing.T) {
	logger := log.Default()
	tam, err := NewTAM(false, nil, logger)
	if err != nil {
		t.Fatalf("NewTAM error: %v", err)
	}
	err = tam.Init()
	if err != nil {
		t.Fatalf("TAM Init error: %v", err)
	}

	// get without set must fail
	_, err = tam.getTEEPAgentKey(make([]byte, 32))
	assert.NotNil(t, err)

	// set and get must success
	err = tam.setTEEPAgentKey(&key, nil)
	assert.Nil(t, err)
	kid, err := key.Thumbprint(crypto.SHA256)
	assert.Nil(t, err)
	k, err := tam.getTEEPAgentKey(kid)
	assert.Equal(t, key, *k)

	// get with nil must fail
	_, err = tam.getTEEPAgentKey(nil)
	assert.NotNil(t, err)
}
