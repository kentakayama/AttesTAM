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
