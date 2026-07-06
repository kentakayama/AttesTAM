/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractQuoteReportDataV3(t *testing.T) {
	quote := make([]byte, sgxQuote3Size)
	quote[0] = quoteVersion3
	for i := 0; i < quoteReportDataSize; i++ {
		quote[sgxReportDataOffsetInQuote+i] = byte(i)
	}

	got, err := extractQuoteReportData(quote)
	require.NoError(t, err)
	require.Len(t, got, quoteReportDataSize)
	for i := 0; i < quoteReportDataSize; i++ {
		require.Equal(t, byte(i), got[i])
	}
}

func TestExtractQuoteReportDataV5Type1(t *testing.T) {
	quote := make([]byte, sgxQuote5HeaderSize+sgxReportBodySize)
	quote[0] = quoteVersion5
	quote[sgxQuote5TypeOffset] = 1
	binary.LittleEndian.PutUint32(quote[sgxQuote5BodyLenOff:sgxQuote5BodyLenOff+4], sgxReportBodySize)
	for i := 0; i < quoteReportDataSize; i++ {
		quote[sgxQuote5HeaderSize+sgxReportDataOffsetInBody+i] = byte(255 - i)
	}

	got, err := extractQuoteReportData(quote)
	require.NoError(t, err)
	require.Len(t, got, quoteReportDataSize)
	for i := 0; i < quoteReportDataSize; i++ {
		require.Equal(t, byte(255-i), got[i])
	}
}

func TestVerifyQuoteReportDataMismatch(t *testing.T) {
	quote := make([]byte, sgxQuote3Size)
	quote[0] = quoteVersion3

	err := verifyQuoteReportData(quote, make([]byte, quoteReportDataSize))
	require.NoError(t, err)

	expected := make([]byte, quoteReportDataSize)
	expected[0] = 1
	err = verifyQuoteReportData(quote, expected)
	require.ErrorContains(t, err, "report_data mismatch")
}

func TestIntelQVLFMSPCFormatting(t *testing.T) {
	fmspc := []byte{0x00, 0x60, 0x6a, 0x00, 0x00, 0x00}

	queryValue, err := intelQVLFMSPCQueryValue(fmspc)
	require.NoError(t, err)
	require.Equal(t, "00606A000000", queryValue)

	cacheKeyValue, err := intelQVLFMSPCCacheKeyValue(fmspc)
	require.NoError(t, err)
	require.Equal(t, "00606a000000", cacheKeyValue)
}
