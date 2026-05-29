/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"crypto"
	"crypto/sha256"
	"crypto/sha512"
	"os"
	"path/filepath"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func TestDecodeIntelQVLAttestationPayload_Array(t *testing.T) {
	payload := intelQVLAttestationPayload{
		Quote:         []byte{0x01, 0x02, 0x03},
		RawReportData: []byte{0xa1, 0x01, 0x02},
	}
	encoded, err := cbor.Marshal(payload)
	require.NoError(t, err)

	decoded, err := decodeIntelQVLAttestationPayload(encoded)
	require.NoError(t, err)
	require.Equal(t, payload.Quote, decoded.Quote)
	require.Equal(t, payload.RawReportData, decoded.RawReportData)
}

func TestIntelQVLAttestationPayload_CBORRoundTrip(t *testing.T) {
	payload := intelQVLAttestationPayload{
		Quote:         []byte{0x01, 0x02, 0x03},
		RawReportData: []byte{0xa1, 0x01, 0x02},
	}
	encoded, err := cbor.Marshal(payload)
	require.NoError(t, err)

	var decoded intelQVLAttestationPayload
	err = cbor.Unmarshal(encoded, &decoded)
	require.NoError(t, err)
	require.Equal(t, payload.Quote, decoded.Quote)
	require.Equal(t, payload.RawReportData, decoded.RawReportData)
}

func TestDecodeIntelQVLAttestationPayload_MissingRawReportDataFallsBackToRawQuote(t *testing.T) {
	encoded, err := cbor.Marshal([][]byte{
		[]byte{0x01, 0x02, 0x03},
	})
	require.NoError(t, err)

	decoded, err := decodeIntelQVLAttestationPayload(encoded)
	require.NoError(t, err)
	require.Equal(t, encoded, decoded.Quote)
	require.Nil(t, decoded.RawReportData)
}

func TestDecodeIntelQVLAttestationPayload_RawQuoteFallback(t *testing.T) {
	decoded, err := decodeIntelQVLAttestationPayload([]byte{0x03, 0x00, 0x02, 0x00})
	require.NoError(t, err)
	require.Equal(t, []byte{0x03, 0x00, 0x02, 0x00}, decoded.Quote)
	require.Nil(t, decoded.RawReportData)
}

func TestBuildIntelQVLExpectedReportData(t *testing.T) {
	raw := []byte{0xa1, 0x01, 0x02}
	expected, err := buildIntelQVLExpectedReportData(raw)
	require.NoError(t, err)
	require.Len(t, expected, 64)

	sum := sha256.Sum256(raw)
	require.Equal(t, sum[:], expected[:sha256.Size])
	for _, b := range expected[sha256.Size:] {
		require.Zero(t, b)
	}
}

func TestBuildIntelQVLExpectedReportData_SHA384(t *testing.T) {
	raw := []byte{0xa1, 0x01, 0x02}
	expected, err := buildIntelQVLExpectedReportDataWithDigest(raw, crypto.SHA384)
	require.NoError(t, err)
	require.Len(t, expected, 64)

	sum := sha512.Sum384(raw)
	require.Equal(t, sum[:], expected[:sha512.Size384])
	for _, b := range expected[sha512.Size384:] {
		require.Zero(t, b)
	}
}

func TestBuildIntelQVLExpectedReportData_FixtureBundleMatchesQuote_SHA384(t *testing.T) {
	bundlePath := filepath.Join("testdata", "sgx-quote3-teep-bundle.cbor")

	bundleBytes, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	var bundle intelQVLAttestationPayload
	err = cbor.Unmarshal(bundleBytes, &bundle)
	require.NoError(t, err)

	expected, err := buildIntelQVLExpectedReportDataWithDigest(bundle.RawReportData, crypto.SHA384)
	require.NoError(t, err)

	err = verifyQuoteReportData(bundle.Quote, expected)
	require.NoError(t, err)
}

func TestClassifyIntelQVLResult(t *testing.T) {
	tests := []struct {
		name                       string
		verificationResult         uint32
		collateralExpirationStatus uint32
		want                       string
	}{
		{
			name:               "ok",
			verificationResult: intelQVLResultOK,
			want:               intelQVLClassificationOK,
		},
		{
			name:               "config needed warning",
			verificationResult: intelQVLResultConfigNeeded,
			want:               intelQVLClassificationOKWithWarnings,
		},
		{
			name:               "out of date warning",
			verificationResult: intelQVLResultOutOfDate,
			want:               intelQVLClassificationOKWithWarnings,
		},
		{
			name:               "out of date config needed warning",
			verificationResult: intelQVLResultOutOfDateConfigNeeded,
			want:               intelQVLClassificationOKWithWarnings,
		},
		{
			name:               "software hardening warning",
			verificationResult: intelQVLResultSWHardeningNeeded,
			want:               intelQVLClassificationOKWithWarnings,
		},
		{
			name:               "config and software hardening warning",
			verificationResult: intelQVLResultConfigAndSWHardeningNeeded,
			want:               intelQVLClassificationOKWithWarnings,
		},
		{
			name:               "td relaunch warning",
			verificationResult: intelQVLResultTDRelaunchAdvised,
			want:               intelQVLClassificationOKWithWarnings,
		},
		{
			name:               "td relaunch config needed warning",
			verificationResult: intelQVLResultTDRelaunchAdvisedConfigNeeded,
			want:               intelQVLClassificationOKWithWarnings,
		},
		{
			name:                       "expired collateral fails ok result",
			verificationResult:         intelQVLResultOK,
			collateralExpirationStatus: 1,
			want:                       intelQVLClassificationFailure,
		},
		{
			name:               "invalid signature failure",
			verificationResult: intelQVLResultInvalidSignature,
			want:               intelQVLClassificationFailure,
		},
		{
			name:               "revoked failure",
			verificationResult: intelQVLResultRevoked,
			want:               intelQVLClassificationFailure,
		},
		{
			name:               "unspecified failure",
			verificationResult: intelQVLResultUnspecified,
			want:               intelQVLClassificationFailure,
		},
		{
			name:               "unknown failure",
			verificationResult: 0xdeadbeef,
			want:               intelQVLClassificationFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyIntelQVLResult(tt.verificationResult, tt.collateralExpirationStatus)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsIntelQVLResultAffirmingValue(t *testing.T) {
	require.True(t, isIntelQVLResultAffirmingValue(intelQVLResultOK, 0))
	require.True(t, isIntelQVLResultAffirmingValue(intelQVLResultConfigNeeded, 0))
	require.False(t, isIntelQVLResultAffirmingValue(intelQVLResultOK, 1))
	require.False(t, isIntelQVLResultAffirmingValue(intelQVLResultRevoked, 0))
}

func TestIntelQVLVerificationResultName(t *testing.T) {
	tests := []struct {
		name               string
		verificationResult uint32
		want               string
	}{
		{
			name:               "ok",
			verificationResult: intelQVLResultOK,
			want:               "TEE_QV_RESULT_OK",
		},
		{
			name:               "config needed",
			verificationResult: intelQVLResultConfigNeeded,
			want:               "TEE_QV_RESULT_CONFIG_NEEDED",
		},
		{
			name:               "revoked",
			verificationResult: intelQVLResultRevoked,
			want:               "TEE_QV_RESULT_REVOKED",
		},
		{
			name:               "unknown",
			verificationResult: 0xdeadbeef,
			want:               "TEE_QV_RESULT_UNKNOWN(0xdeadbeef)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intelQVLVerificationResultName(tt.verificationResult)
			require.Equal(t, tt.want, got)
		})
	}
}
