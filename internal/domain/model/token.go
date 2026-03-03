/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import "time"

// Token represents a one-time token for TEEP protocol messages.
type Token struct {
	ID        int64
	Token     []byte
	CreatedAt time.Time
	ExpiredAt time.Time
	Consumed  bool
}
