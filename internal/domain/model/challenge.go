/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import "time"

// Challenge represents a challenge for remote attestation.
type Challenge struct {
	ID        int64
	Challenge []byte
	CreatedAt time.Time
	ExpiredAt time.Time
	Consumed  bool
}
