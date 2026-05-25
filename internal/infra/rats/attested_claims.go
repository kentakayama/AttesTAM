/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"errors"
	"fmt"

	"github.com/veraison/eat"
	"github.com/veraison/go-cose"
)

func PopulateAttestedClaimsFromEAT(result *ProcessedAttestation, raw []byte) error {
	if result == nil {
		return errors.New("processed attestation is nil")
	}

	var evidence eat.Eat
	if err := evidence.FromCBOR(raw); err != nil {
		return fmt.Errorf("decode EAT claims: %w", err)
	}
	if err := attachAttestedClaims(result, &evidence); err != nil {
		return err
	}

	return nil
}

func PopulateAttestedClaimsFromSign1(result *ProcessedAttestation, payload []byte) error {
	if result == nil {
		return errors.New("processed attestation is nil")
	}

	var sign1 cose.Sign1Message
	if err := sign1.UnmarshalCBOR(payload); err != nil {
		return fmt.Errorf("decode attestation Sign1: %w", err)
	}
	return PopulateAttestedClaimsFromEAT(result, sign1.Payload)
}

func attachAttestedClaims(result *ProcessedAttestation, evidence *eat.Eat) error {
	if evidence == nil {
		return errors.New("EAT evidence is nil")
	}
	if evidence.Nonce == nil {
		return errors.New("EAT nonce missing")
	}
	if err := evidence.Nonce.Validate(); err != nil {
		return fmt.Errorf("validate EAT nonce: %w", err)
	}
	if evidence.Nonce.Len() != 1 {
		return errors.New("EAT nonce must contain exactly one challenge")
	}
	if evidence.Cnf == nil || evidence.Cnf.Key == nil {
		return errors.New("EAT cnf key missing")
	}

	result.AttestationNonce = evidence.Nonce.GetI(0)
	result.AttestationKey = evidence.Cnf.Key
	if evidence.UEID != nil && evidence.UEID.Validate() == nil {
		result.AttestationUEID = []byte(*evidence.UEID)
	}

	return nil
}
