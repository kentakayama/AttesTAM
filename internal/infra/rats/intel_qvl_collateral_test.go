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
	"time"

	"github.com/stretchr/testify/require"
)

func TestIntelQVLCollateralCacheKeyFromFixture(t *testing.T) {
	quote := readSGXQuoteFixture(t)
	pckCA, err := extractIntelQVLQuotePCKCA(quote)
	require.NoError(t, err)

	key, err := intelQVLCollateralCacheKey(quote)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(key, "fmspc-"), "cache key = %q", key)
	require.Contains(t, key, "-pckca-"+pckCA)
	require.Len(t, strings.TrimSuffix(strings.TrimPrefix(key, "fmspc-"), "-pckca-"+pckCA), intelQVLFMSPCSize*2)
}

func TestIntelQVLCollateralCacheRoundTrip(t *testing.T) {
	quote := readSGXQuoteFixture(t)
	cache, err := newIntelQVLCollateralCache(t.TempDir(), nil)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC)
	origNow := intelQVLNow
	intelQVLNow = func() time.Time { return now }
	t.Cleanup(func() { intelQVLNow = origNow })

	collateral := &intelQVLQuoteCollateral{
		MajorVersion:          intelQVLPCSAPIVersionMajor,
		MinorVersion:          intelQVLPCSCRLVersionMinor,
		PCKCRLIssuerChain:     []byte("pck-crl-issuer"),
		TCBInfoIssuerChain:    []byte("tcb-info-issuer"),
		TCBInfo:               []byte(`{"tcbInfo":{"id":"SGX"},"nextUpdate":"2026-07-05T00:00:00Z"}`),
		QEIdentityIssuerChain: []byte("qe-identity-issuer"),
		QEIdentity:            []byte(`{"enclaveIdentity":{"id":"QE"},"nextUpdate":"2026-07-06T00:00:00Z"}`),
	}
	key, err := cache.Put(quote, collateral)
	require.NoError(t, err)
	require.NotEmpty(t, key)

	got, err := cache.Get(quote)
	require.NoError(t, err)
	require.Equal(t, collateral, got)
}

func TestIntelQVLCollateralCacheGetExpiredTreatsAsMiss(t *testing.T) {
	quote := readSGXQuoteFixture(t)
	cache, err := newIntelQVLCollateralCache(t.TempDir(), nil)
	require.NoError(t, err)

	now := time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC)
	origNow := intelQVLNow
	intelQVLNow = func() time.Time { return now }
	t.Cleanup(func() { intelQVLNow = origNow })

	collateral := &intelQVLQuoteCollateral{
		MajorVersion: intelQVLPCSAPIVersionMajor,
		MinorVersion: intelQVLPCSCRLVersionMinor,
		TCBInfo:      []byte(`{"nextUpdate":"2026-07-03T00:00:00Z"}`),
	}
	_, err = cache.Put(quote, collateral)
	require.NoError(t, err)

	intelQVLNow = func() time.Time { return now.Add(48 * time.Hour) }
	got, err := cache.Get(quote)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestIntelQVLQuoteCollateralExpiryUsesEarlierOfTTLAndNextUpdate(t *testing.T) {
	now := time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC)

	ttlOnly, err := intelQVLQuoteCollateralExpiry(&intelQVLQuoteCollateral{}, now)
	require.NoError(t, err)
	require.Equal(t, now.Add(intelQVLCollateralCacheTTL), ttlOnly)

	withEarlierNextUpdate, err := intelQVLQuoteCollateralExpiry(&intelQVLQuoteCollateral{
		TCBInfo: []byte(`{"nextUpdate":"2026-07-04T12:00:00Z"}`),
	}, now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC), withEarlierNextUpdate)
}

func readSGXQuoteFixture(t *testing.T) []byte {
	t.Helper()

	quote, err := os.ReadFile(filepath.Join("testdata", "quote.bin"))
	require.NoError(t, err)
	return quote
}
