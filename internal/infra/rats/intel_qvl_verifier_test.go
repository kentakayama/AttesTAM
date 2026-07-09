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
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestIntelQVLVerifierProcess_QuoteFixtureCachesCollateral(t *testing.T) {
	quote := readIntelQVLQuoteFixture(t)
	cache, err := newIntelQVLCollateralCache(t.TempDir(), nil)
	require.NoError(t, err)

	origVerify := intelQVLQuoteVerifier
	origCollateralGetter := intelQVLQuoteCollateralGetter
	t.Cleanup(func() {
		intelQVLQuoteVerifier = origVerify
		intelQVLQuoteCollateralGetter = origCollateralGetter
	})

	const expectedCollateral = "fixture-qvl-collateral"
	var collateralCalls int
	var verifyCalls int

	intelQVLQuoteCollateralGetter = func(_ *intelQVLPCSClient, gotQuote []byte, cQuote unsafe.Pointer, quoteSize int) (*intelQVLQuoteCollateral, error) {
		collateralCalls++
		require.Equal(t, len(quote), quoteSize)
		require.Equal(t, quote, gotQuote)
		require.Equal(t, quote, append([]byte(nil), unsafe.Slice((*byte)(cQuote), quoteSize)...))
		return &intelQVLQuoteCollateral{
			MajorVersion: intelQVLPCSAPIVersionMajor,
			MinorVersion: intelQVLPCSCRLVersionMinor,
			TCBInfo:      []byte(`{"nextUpdate":"2030-01-01T00:00:00Z","opaque":"` + expectedCollateral + `"}`),
		}, nil
	}
	intelQVLQuoteVerifier = func(cQuote unsafe.Pointer, quoteSize int, collateral *intelQVLQuoteCollateral) (uint32, intelQVLNativeResult) {
		verifyCalls++
		require.Equal(t, len(quote), quoteSize)
		require.Equal(t, quote, append([]byte(nil), unsafe.Slice((*byte)(cQuote), quoteSize)...))
		require.Contains(t, string(collateral.TCBInfo), expectedCollateral)
		return 0, intelQVLNativeResult{
			verificationResult: intelQVLResultOK,
		}
	}

	verifier := &IntelQVLVerifier{collateralCache: cache}

	first, err := verifier.Process(quote)
	require.NoError(t, err)
	require.Equal(t, VerifierBackendIntelQVL, first.Backend)
	require.Equal(t, "affirming", first.EarStatus)
	require.False(t, first.SendUpdate)
	require.Equal(t, 1, collateralCalls)
	require.Equal(t, 1, verifyCalls)

	second, err := verifier.Process(quote)
	require.NoError(t, err)
	require.Equal(t, VerifierBackendIntelQVL, second.Backend)
	require.Equal(t, "affirming", second.EarStatus)
	require.False(t, second.SendUpdate)
	require.Equal(t, 1, collateralCalls)
	require.Equal(t, 2, verifyCalls)
}

func TestIntelQVLVerifierProcess_CachesCollateralBeforeVerificationFailure(t *testing.T) {
	quote := readIntelQVLQuoteFixture(t)
	cache, err := newIntelQVLCollateralCache(t.TempDir(), nil)
	require.NoError(t, err)

	origVerify := intelQVLQuoteVerifier
	origCollateralGetter := intelQVLQuoteCollateralGetter
	t.Cleanup(func() {
		intelQVLQuoteVerifier = origVerify
		intelQVLQuoteCollateralGetter = origCollateralGetter
	})

	const expectedCollateral = "collateral-for-debugging"
	intelQVLQuoteCollateralGetter = func(_ *intelQVLPCSClient, _ []byte, _ unsafe.Pointer, _ int) (*intelQVLQuoteCollateral, error) {
		return &intelQVLQuoteCollateral{
			MajorVersion: intelQVLPCSAPIVersionMajor,
			MinorVersion: intelQVLPCSCRLVersionMinor,
			TCBInfo:      []byte(`{"nextUpdate":"2030-01-01T00:00:00Z","opaque":"` + expectedCollateral + `"}`),
		}, nil
	}
	intelQVLQuoteVerifier = func(_ unsafe.Pointer, _ int, _ *intelQVLQuoteCollateral) (uint32, intelQVLNativeResult) {
		return 1, intelQVLNativeResult{dcapStatus: 1}
	}

	verifier := &IntelQVLVerifier{collateralCache: cache}

	_, err = verifier.Process(quote)
	require.ErrorContains(t, err, "tee_verify_quote failed")

	collateral, err := cache.Get(quote)
	require.NoError(t, err)
	require.Contains(t, string(collateral.TCBInfo), expectedCollateral)
}

func TestIntelQVLVerifierProcess_AzureQuoteFixtureWithCachedCollateral(t *testing.T) {
	quote, err := os.ReadFile(filepath.Join("testdata", "azure-quote.bin"))
	require.NoError(t, err)
	require.NotEmpty(t, quote)

	cacheDir := t.TempDir()
	cache, err := newIntelQVLCollateralCache(cacheDir, nil)
	require.NoError(t, err)
	key, err := intelQVLCollateralCacheKey(quote)
	require.NoError(t, err)
	collateral, err := os.ReadFile(filepath.Join("testdata", "fmspc-00606a000000-pckca-platform.bin"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, key+".bin"), collateral, 0o600))

	verifier := &IntelQVLVerifier{collateralCache: cache}
	result, err := verifier.Process(quote)
	require.NoError(t, err)
	require.Equal(t, VerifierBackendIntelQVL, result.Backend)
	require.Equal(t, "affirming", result.EarStatus)
	require.False(t, result.SendUpdate)
}

func readIntelQVLQuoteFixture(t *testing.T) []byte {
	t.Helper()

	quote, err := os.ReadFile(filepath.Join("testdata", "quote.bin"))
	require.NoError(t, err)
	require.NotEmpty(t, quote)
	return quote
}
