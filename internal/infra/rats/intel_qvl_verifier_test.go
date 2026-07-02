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
			PCKCRL:       []byte(expectedCollateral),
		}, nil
	}
	intelQVLQuoteVerifier = func(cQuote unsafe.Pointer, quoteSize int, collateral *intelQVLQuoteCollateral) (uint32, intelQVLNativeResult) {
		verifyCalls++
		require.Equal(t, len(quote), quoteSize)
		require.Equal(t, quote, append([]byte(nil), unsafe.Slice((*byte)(cQuote), quoteSize)...))
		require.Equal(t, []byte(expectedCollateral), collateral.PCKCRL)
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

func readIntelQVLQuoteFixture(t *testing.T) []byte {
	t.Helper()

	quote, err := os.ReadFile(filepath.Join("testdata", "quote.bin"))
	require.NoError(t, err)
	require.NotEmpty(t, quote)
	return quote
}
