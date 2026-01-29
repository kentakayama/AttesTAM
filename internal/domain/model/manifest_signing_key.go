/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import "time"

// ManifestSigningKey represents a manifest signing key for a TC Developer.
type ManifestSigningKey struct {
	ID        int64
	KID       []byte
	EntityID  int64
	PublicKey []byte
	CreatedAt time.Time
	ExpiredAt time.Time
}
