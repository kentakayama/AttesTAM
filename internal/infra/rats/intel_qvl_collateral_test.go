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

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

type testSGXQuote3TEEPBundle struct {
	_             struct{} `cbor:",toarray"`
	Quote         []byte
	RawReportData []byte
}

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

	collateral := []byte("opaque-qvl-collateral")
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

	payload, err := os.ReadFile(filepath.Join("testdata", "sgx-quote3-teep-bundle.cbor"))
	require.NoError(t, err)
	var bundle testSGXQuote3TEEPBundle
	require.NoError(t, cbor.Unmarshal(payload, &bundle))
	require.NotEmpty(t, bundle.Quote)
	return bundle.Quote
}
