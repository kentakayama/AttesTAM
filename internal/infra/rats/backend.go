/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"fmt"
	"strings"

	"github.com/kentakayama/AttesTAM/internal/config"
)

const (
	VerifierBackendVeraison               = "veraison"
	VerifierBackendIntelQVL               = "intel-qvl"
	AttestationPayloadFormatSGXQuote3TEEP = "application/sgx-quote3-teep-bundle"
)

func NewVerifier(cfg config.RAConfig) (IRAVerifier, error) {
	backend := verifierBackendFromPayloadFormat(cfg.Backend)
	return NewVerifierForBackend(backend, cfg)
}

func NewVerifierForBackend(backend string, cfg config.RAConfig) (IRAVerifier, error) {
	switch strings.ToLower(backend) {
	case VerifierBackendVeraison:
		return NewVerifierClient(cfg)
	case VerifierBackendIntelQVL:
		return NewIntelQVLVerifier(cfg)
	default:
		return nil, fmt.Errorf("unsupported verifier backend %q", backend)
	}
}

func VerifierBackendForAttestationPayloadFormat(format string) string {
	if strings.EqualFold(strings.TrimSpace(format), AttestationPayloadFormatSGXQuote3TEEP) {
		return VerifierBackendIntelQVL
	}
	return VerifierBackendVeraison
}

func verifierBackendFromPayloadFormat(format string) string {
	trimmed := strings.TrimSpace(format)
	if trimmed == "" {
		return VerifierBackendVeraison
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case VerifierBackendVeraison, VerifierBackendIntelQVL:
		return lower
	default:
		return VerifierBackendForAttestationPayloadFormat(trimmed)
	}
}
