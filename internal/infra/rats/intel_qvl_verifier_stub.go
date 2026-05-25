//go:build !intel_qvl || !cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"fmt"

	"github.com/kentakayama/AttesTAM/internal/config"
)

func NewIntelQVLVerifier(cfg config.RAConfig) (IRAVerifier, error) {
	return nil, fmt.Errorf("intel-qvl verifier backend requires build tag `intel_qvl` with cgo enabled")
}
