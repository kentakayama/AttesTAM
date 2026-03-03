/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import (
	"time"
)

// AgentStatus represents an agent's attributes such as UEID and possession of a SUIT manifest.
type AgentStatus struct {
	AgentKID      []byte
	DeviceUEID    []byte
	SuitManifests []SuitManifestOverview
	UpdatedAt     time.Time
}
