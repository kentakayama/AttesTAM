/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
