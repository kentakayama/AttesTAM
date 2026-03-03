/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import "time"

// SentQueryRequestMessage represents a QueryRequest message sent by TAM.
type SentQueryRequestMessage struct {
	ID                   int64
	AgentID              *int64 // NULL if no specific agent
	AttestationRequested bool
	TCListRequested      bool
	TokenID              *int64 // NULL if challenge is used
	ChallengeID          *int64 // NULL if token is used
	CreatedAt            time.Time
}
