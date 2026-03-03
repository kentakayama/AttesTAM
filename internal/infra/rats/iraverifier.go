/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

// IRAVerifier defines an interface for Evidence verification
type IRAVerifier interface {
	Process([]byte) (*ProcessedAttestation, error)
}
