//go:build !intel_qvl || !cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import "fmt"

func extractIntelQVLFMSPCNative(quote []byte) ([]byte, error) {
	return nil, fmt.Errorf("Intel QVL FMSPC extraction requires build tag `intel_qvl` with cgo enabled")
}
