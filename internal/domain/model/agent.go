/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import "time"

type Agent struct {
	ID        int64
	KID       []byte // Primary Key
	DeviceID  *int64
	CreatedAt time.Time
	ExpiredAt time.Time  // from eat['exp']
	RevokedAt *time.Time // NULL if not revoked
	PublicKey []byte     // COSE_Key
}
