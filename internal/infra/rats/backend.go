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
	VerifierBackendVeraison = "veraison"
	VerifierBackendIntelQVL = "intel-qvl"
)

func NewVerifier(cfg config.RAConfig) (IRAVerifier, error) {
	backend := strings.TrimSpace(cfg.Backend)
	if backend == "" {
		backend = VerifierBackendVeraison
	}

	switch strings.ToLower(backend) {
	case VerifierBackendVeraison:
		return NewVerifierClient(cfg)
	case VerifierBackendIntelQVL:
		return NewIntelQVLVerifier(cfg)
	default:
		return nil, fmt.Errorf("unsupported verifier backend %q", backend)
	}
}
