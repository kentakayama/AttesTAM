/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"crypto/sha256"
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

func TestBuildIntelQVLExpectedReportData_FixtureMatchesQuote(t *testing.T) {
	reportDataPath := filepath.Join("testdata", "report_data.dat")
	quotePath := filepath.Join("testdata", "quote.dat")

	rawReportData, err := os.ReadFile(reportDataPath)
	require.NoError(t, err)
	quote, err := os.ReadFile(quotePath)
	require.NoError(t, err)

	expected, err := buildIntelQVLExpectedReportData(rawReportData)
	require.NoError(t, err)

	err = verifyQuoteReportData(quote, expected)
	require.NoError(t, err)
}
