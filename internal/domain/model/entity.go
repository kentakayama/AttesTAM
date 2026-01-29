/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import "time"

// Entity represents either TAM Admin, Trusted Component Developer or Device Admin.
type Entity struct {
	ID   int64
	Name string
	// TODO? Create **RICH** authorization management
	IsTAMAdmin    bool
	IsTCDeveloper bool
	IsDeviceAdmin bool
	CreatedAt     time.Time
}
