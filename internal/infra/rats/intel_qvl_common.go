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
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/veraison/go-cose"
)

const intelQVLNonAffirming = "non-affirming"

const (
	intelQVLClassificationOK             = "ok"
	intelQVLClassificationOKWithWarnings = "ok-with-warnings"
	intelQVLClassificationFailure        = "failure"
)

const (
	intelQVLResultOK                            uint32 = 0x0000
	intelQVLResultConfigNeeded                  uint32 = 0x0000a001
	intelQVLResultOutOfDate                     uint32 = 0x0000a002
	intelQVLResultOutOfDateConfigNeeded         uint32 = 0x0000a003
	intelQVLResultInvalidSignature              uint32 = 0x0000a004
	intelQVLResultRevoked                       uint32 = 0x0000a005
	intelQVLResultUnspecified                   uint32 = 0x0000a006
	intelQVLResultSWHardeningNeeded             uint32 = 0x0000a007
	intelQVLResultConfigAndSWHardeningNeeded    uint32 = 0x0000a008
	intelQVLResultTDRelaunchAdvised             uint32 = 0x0000a009
	intelQVLResultTDRelaunchAdvisedConfigNeeded uint32 = 0x0000a00a
)

const (
	intelQVLRawReportDataEC2XOffset = 0
	intelQVLRawReportDataEC2YOffset = 32
	intelQVLRawReportDataEC2Size    = 64
)

type intelQVLAttestationPayload struct {
	_             struct{} `cbor:",toarray"`
	Quote         []byte
	RawReportData []byte
}

func decodeIntelQVLAttestationPayload(payload []byte) (*intelQVLAttestationPayload, error) {
	if len(payload) == 0 {
		return nil, errors.New("attestation payload is empty")
	}

	var decoded intelQVLAttestationPayload
	err := cbor.Unmarshal(payload, &decoded)
	if err == nil {
		if len(decoded.Quote) == 0 {
			return nil, errors.New("SGX attestation payload missing quote")
		}
		return &decoded, nil
	}

	// Fallback to treating the payload itself as a raw quote.
	return &intelQVLAttestationPayload{Quote: payload}, nil
}

func buildIntelQVLExpectedReportData(rawReportData []byte) ([]byte, error) {
	return buildIntelQVLExpectedReportDataWithDigest(rawReportData, crypto.SHA256)
}

func buildIntelQVLExpectedReportDataWithDigest(rawReportData []byte, digest crypto.Hash) ([]byte, error) {
	if len(rawReportData) == 0 {
		return nil, errors.New("raw-report-data is empty")
	}

	expected := make([]byte, 64)
	switch digest {
	case crypto.SHA256:
		sum := sha256.Sum256(rawReportData)
		copy(expected, sum[:])
	case crypto.SHA384:
		sum := sha512.Sum384(rawReportData)
		copy(expected, sum[:])
	default:
		return nil, fmt.Errorf("unsupported digest %v", digest)
	}
	return expected, nil
}

func resultToEarStatus(ok bool) string {
	if ok {
		return "affirming"
	}
	return intelQVLNonAffirming
}

func classifyIntelQVLResult(verificationResult uint32, collateralExpirationStatus uint32) string {
	if collateralExpirationStatus != 0 {
		return intelQVLClassificationFailure
	}

	switch verificationResult {
	case intelQVLResultOK:
		return intelQVLClassificationOK
	case intelQVLResultConfigNeeded,
		intelQVLResultOutOfDate,
		intelQVLResultOutOfDateConfigNeeded,
		intelQVLResultSWHardeningNeeded,
		intelQVLResultConfigAndSWHardeningNeeded,
		intelQVLResultTDRelaunchAdvised,
		intelQVLResultTDRelaunchAdvisedConfigNeeded:
		return intelQVLClassificationOKWithWarnings
	default:
		return intelQVLClassificationFailure
	}
}

func isIntelQVLResultAffirmingValue(verificationResult uint32, collateralExpirationStatus uint32) bool {
	classification := classifyIntelQVLResult(verificationResult, collateralExpirationStatus)
	return classification == intelQVLClassificationOK || classification == intelQVLClassificationOKWithWarnings
}

func intelQVLVerificationResultName(verificationResult uint32) string {
	switch verificationResult {
	case intelQVLResultOK:
		return "TEE_QV_RESULT_OK"
	case intelQVLResultConfigNeeded:
		return "TEE_QV_RESULT_CONFIG_NEEDED"
	case intelQVLResultOutOfDate:
		return "TEE_QV_RESULT_OUT_OF_DATE"
	case intelQVLResultOutOfDateConfigNeeded:
		return "TEE_QV_RESULT_OUT_OF_DATE_CONFIG_NEEDED"
	case intelQVLResultInvalidSignature:
		return "TEE_QV_RESULT_INVALID_SIGNATURE"
	case intelQVLResultRevoked:
		return "TEE_QV_RESULT_REVOKED"
	case intelQVLResultUnspecified:
		return "TEE_QV_RESULT_UNSPECIFIED"
	case intelQVLResultSWHardeningNeeded:
		return "TEE_QV_RESULT_SW_HARDENING_NEEDED"
	case intelQVLResultConfigAndSWHardeningNeeded:
		return "TEE_QV_RESULT_CONFIG_AND_SW_HARDENING_NEEDED"
	case intelQVLResultTDRelaunchAdvised:
		return "TEE_QV_RESULT_TD_RELAUNCH_ADVISED"
	case intelQVLResultTDRelaunchAdvisedConfigNeeded:
		return "TEE_QV_RESULT_TD_RELAUNCH_ADVISED_CONFIG_NEEDED"
	default:
		return fmt.Sprintf("TEE_QV_RESULT_UNKNOWN(0x%x)", verificationResult)
	}
}

func validateIntelQVLClaims(rawReportData []byte, result *ProcessedAttestation) error {
	if len(rawReportData) == 0 {
		return errors.New("raw-report-data is required for Intel QVL attestation")
	}
	if err := PopulateAttestedClaims(result, rawReportData); err == nil {
		return nil
	}
	if err := populateAttestedKeyFromIntelQVLRawReportData(result, rawReportData); err != nil {
		return fmt.Errorf("extract attested claims from raw-report-data: %w", err)
	}
	return nil
}

func populateAttestedKeyFromIntelQVLRawReportData(result *ProcessedAttestation, rawReportData []byte) error {
	if result == nil {
		return errors.New("processed attestation is nil")
	}
	if len(rawReportData) < intelQVLRawReportDataEC2Size {
		return fmt.Errorf("raw-report-data too short for EC2 key: got %d bytes", len(rawReportData))
	}

	key, err := cose.NewKeyEC2(
		cose.AlgorithmESP256,
		rawReportData[intelQVLRawReportDataEC2XOffset:intelQVLRawReportDataEC2YOffset],
		rawReportData[intelQVLRawReportDataEC2YOffset:intelQVLRawReportDataEC2Size],
		nil,
	)
	if err != nil {
		return fmt.Errorf("build attested EC2 key: %w", err)
	}
	result.AttestationKey = key
	if len(rawReportData) > intelQVLRawReportDataEC2Size {
		result.AttestationNonce = append([]byte(nil), rawReportData[intelQVLRawReportDataEC2Size:]...)
	}
	return nil
}
