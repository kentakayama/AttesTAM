/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"bytes"
	"crypto/sha512"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/AttesTAM/internal/infra/rats"
	"github.com/veraison/go-cose"
)

const (
	sgxRawReportDataEC2XOffset = 0
	sgxRawReportDataEC2YOffset = 32
	sgxRawReportDataEC2Size    = 64
)

type sgxQuote3TEEPBundle struct {
	_             struct{} `cbor:",toarray"`
	Quote         []byte
	RawReportData []byte
}

func decodeSGXQuote3TEEPBundle(payload []byte) (*sgxQuote3TEEPBundle, error) {
	if len(payload) == 0 {
		return nil, errors.New("attestation payload is empty")
	}

	var decoded sgxQuote3TEEPBundle
	if err := cbor.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("decode SGX Quote3 TEEP bundle: %w", err)
	}
	if len(decoded.Quote) == 0 {
		return nil, errors.New("SGX Quote3 TEEP bundle missing quote")
	}
	if len(decoded.RawReportData) == 0 {
		return nil, errors.New("SGX Quote3 TEEP bundle missing raw-report-data")
	}
	return &decoded, nil
}

func verifySGXQuoteReportDataBinding(quote, rawReportData []byte) error {
	if len(rawReportData) == 0 {
		return errors.New("raw-report-data is empty")
	}

	actual, err := rats.ExtractQuoteReportData(quote)
	if err != nil {
		return err
	}

	expected := make([]byte, 64)
	sum := sha512.Sum384(rawReportData)
	copy(expected, sum[:])
	if !bytes.Equal(actual, expected) {
		return errors.New("quote report_data SHA-384 binding mismatch")
	}
	return nil
}

func populateAttestedClaimsFromSGXRawReportData(result *rats.ProcessedAttestation, rawReportData []byte) error {
	if result == nil {
		return errors.New("processed attestation is nil")
	}
	if len(rawReportData) == 0 {
		return errors.New("raw-report-data is required for SGX attestation")
	}
	if err := populateAttestedKeyFromSGXRawReportData(result, rawReportData); err != nil {
		return fmt.Errorf("extract attested claims from raw-report-data: %w", err)
	}
	return nil
}

func populateAttestedKeyFromSGXRawReportData(result *rats.ProcessedAttestation, rawReportData []byte) error {
	if len(rawReportData) < sgxRawReportDataEC2Size {
		return fmt.Errorf("raw-report-data too short for EC2 key: got %d bytes", len(rawReportData))
	}

	key, err := cose.NewKeyEC2(
		cose.AlgorithmESP256,
		rawReportData[sgxRawReportDataEC2XOffset:sgxRawReportDataEC2YOffset],
		rawReportData[sgxRawReportDataEC2YOffset:sgxRawReportDataEC2Size],
		nil,
	)
	if err != nil {
		return fmt.Errorf("build attested EC2 key: %w", err)
	}
	result.AttestationKey = key
	if len(rawReportData) > sgxRawReportDataEC2Size {
		result.AttestationNonce = append([]byte(nil), rawReportData[sgxRawReportDataEC2Size:]...)
	}
	return nil
}
