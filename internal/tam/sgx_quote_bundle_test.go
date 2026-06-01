/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/AttesTAM/internal/infra/rats"
	"github.com/stretchr/testify/require"
	"github.com/veraison/go-cose"
)

func TestDecodeSGXQuote3TEEPBundle(t *testing.T) {
	fixture := readSGXQuote3TEEPBundleFixture(t)

	decoded, err := decodeSGXQuote3TEEPBundle(fixture)
	require.NoError(t, err)
	require.NotEmpty(t, decoded.Quote)
	require.NotEmpty(t, decoded.RawReportData)
}

func TestDecodeSGXQuote3TEEPBundleMissingRawReportData(t *testing.T) {
	payload, err := cbor.Marshal(sgxQuote3TEEPBundle{
		Quote: []byte{0x03, 0x00},
	})
	require.NoError(t, err)

	_, err = decodeSGXQuote3TEEPBundle(payload)
	require.ErrorContains(t, err, "raw-report-data")
}

func TestVerifySGXQuoteReportDataBinding(t *testing.T) {
	fixture := readSGXQuote3TEEPBundleFixture(t)
	decoded, err := decodeSGXQuote3TEEPBundle(fixture)
	require.NoError(t, err)

	err = verifySGXQuoteReportDataBinding(decoded.Quote, decoded.RawReportData)
	require.NoError(t, err)
}

func TestVerifySGXQuoteReportDataBindingMismatch(t *testing.T) {
	fixture := readSGXQuote3TEEPBundleFixture(t)
	decoded, err := decodeSGXQuote3TEEPBundle(fixture)
	require.NoError(t, err)

	rawReportData := append([]byte(nil), decoded.RawReportData...)
	rawReportData[0] ^= 0xff

	err = verifySGXQuoteReportDataBinding(decoded.Quote, rawReportData)
	require.ErrorContains(t, err, "report_data")
}

func TestPopulateAttestedClaimsFromSGXRawReportData(t *testing.T) {
	fixture := readSGXQuote3TEEPBundleFixture(t)
	decoded, err := decodeSGXQuote3TEEPBundle(fixture)
	require.NoError(t, err)

	result := &rats.ProcessedAttestation{}
	err = populateAttestedClaimsFromSGXRawReportData(result, decoded.RawReportData)
	require.NoError(t, err)
	require.NotNil(t, result.AttestationKey)
	require.NotEmpty(t, result.AttestationNonce)

	curve, x, y, d := result.AttestationKey.EC2()
	require.Equal(t, cose.CurveP256, curve)
	require.Len(t, x, 32)
	require.Len(t, y, 32)
	require.Empty(t, d)
}

func readSGXQuote3TEEPBundleFixture(t *testing.T) []byte {
	t.Helper()

	fixturePath := filepath.Join("testdata", "sgx_quote3_teep_bundle.cbor")
	fixture, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	return fixture
}
