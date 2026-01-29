/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import "time"

type Device struct {
	ID        int64
	UEID      []byte
	AdminID   int64
	CreatedAt time.Time
}
