/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package domain

import "errors"

var (
	ErrNotFound = errors.New("item not found")
	ErrExpired  = errors.New("item expired")
	ErrRevoked  = errors.New("item revoked")
)
