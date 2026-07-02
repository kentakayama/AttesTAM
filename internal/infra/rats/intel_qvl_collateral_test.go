//go:build intel_qvl && cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntelQVLCollateralCacheKeyFromFixture(t *testing.T) {
	quote := readSGXQuoteFixture(t)

	key, err := intelQVLCollateralCacheKey(quote)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(key, "fmspc-"), "cache key = %q", key)
	require.Len(t, strings.TrimPrefix(key, "fmspc-"), intelQVLFMSPCSize*2)
}

func TestIntelQVLCollateralCacheRoundTrip(t *testing.T) {
	quote := readSGXQuoteFixture(t)
	cache, err := newIntelQVLCollateralCache(t.TempDir(), nil)
	require.NoError(t, err)

	collateral := &intelQVLQuoteCollateral{
		MajorVersion:          intelQVLPCSAPIVersionMajor,
		MinorVersion:          intelQVLPCSCRLVersionMinor,
		PCKCRLIssuerChain:     []byte("pck-crl-issuer"),
		RootCACRL:             []byte("root-ca-crl"),
		PCKCRL:                []byte("pck-crl"),
		TCBInfoIssuerChain:    []byte("tcb-info-issuer"),
		TCBInfo:               []byte(`{"tcbInfo":{"id":"SGX"}}`),
		QEIdentityIssuerChain: []byte("qe-identity-issuer"),
		QEIdentity:            []byte(`{"enclaveIdentity":{"id":"QE"}}`),
	}
	key, err := cache.Put(quote, collateral)
	require.NoError(t, err)
	require.NotEmpty(t, key)

	got, ok, gotKey, err := cache.Get(quote)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, key, gotKey)
	require.Equal(t, collateral, got)
}

func readSGXQuoteFixture(t *testing.T) []byte {
	t.Helper()

	quote, err := os.ReadFile(filepath.Join("testdata", "quote.bin"))
	require.NoError(t, err)
	return quote
}
