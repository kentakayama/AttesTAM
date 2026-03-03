/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import (
	"time"
)

// SentUpdateMessage represents a Update message sent by TAM.
type SentUpdateMessage struct {
	ID        int64
	AgentID   int64
	TokenID   int64
	CreatedAt time.Time
}

// SentUpdateMessageWithManifests represents a sent update message with its associated SUIT manifests and token.
type SentUpdateMessageWithManifests struct {
	SentUpdateMessage SentUpdateMessage
	Token             Token
	Manifests         []SuitManifest
}
