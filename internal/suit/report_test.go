/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package suit

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSUITReport_UnmarshalOK(t *testing.T) {
	/*
		{
		    / suit-reference / 99: [
		      	/ suit-report-manifest-uri: / "",
		      	/ suit-report-manifest-digest: / [
					-16,
					h'6658EA560262696DD1F13B782239A064DA7C6C5CBAF52FDED428A6FC83C7E5AF'
				]
			],
			/ suit-report-records / 3: [
				/ system-property-claims = / {
					/ system-component-id / 0: [<< 0 >>],
					/ image-size / 14: 34768
				}
			],
			/ suit-report-result / 4: true
		}
	*/
	encoded := []byte{
		0xA3, 0x18, 0x63, 0x82, 0x60, 0x82, 0x2F, 0x58, 0x20, 0x66, 0x58, 0xEA, 0x56, 0x02, 0x62, 0x69,
		0x6D, 0xD1, 0xF1, 0x3B, 0x78, 0x22, 0x39, 0xA0, 0x64, 0xDA, 0x7C, 0x6C, 0x5C, 0xBA, 0xF5, 0x2F,
		0xDE, 0xD4, 0x28, 0xA6, 0xFC, 0x83, 0xC7, 0xE5, 0xAF, 0x03, 0x81, 0xA2, 0x00, 0x81, 0x41, 0x00,
		0x0E, 0x19, 0x87, 0xD0, 0x04, 0xF5,
	}

	var report Report
	if err := cbor.Unmarshal(encoded, &report); err != nil {
		t.Fatalf("Report.UnmarshalCBOR() error = %v", err)
	}

	assert.Equal(t, "", report.Reference.ManifestUri)
	assert.Equal(t, Digest{
		DigestAlg: -16,
		DigestBytes: []byte{
			0x66, 0x58, 0xEA, 0x56, 0x02, 0x62, 0x69, 0x6D, 0xD1, 0xF1, 0x3B, 0x78, 0x22, 0x39, 0xA0, 0x64,
			0xDA, 0x7C, 0x6C, 0x5C, 0xBA, 0xF5, 0x2F, 0xDE, 0xD4, 0x28, 0xA6, 0xFC, 0x83, 0xC7, 0xE5, 0xAF,
		},
	}, report.Reference.Digest)

	assert.Equal(t, 1, len(report.ReportRecords))
	require.NotNil(t, report.ReportRecords[0].SystemClaims)
	assert.Equal(t, &SystemPropertyClaims{
		SystemComponentID: []ComponentIDBytes{{0x00}},
		ImageSize:         34768,
	}, report.ReportRecords[0].SystemClaims)

	require.NotNil(t, report.Result.True)
	assert.Equal(t, true, *report.Result.True)
	require.Nil(t, report.Result.DetailedResult)
}
