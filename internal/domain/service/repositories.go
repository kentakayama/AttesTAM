/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package service

import (
	"context"

	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

// AgentRepository defines the interface for agent persistence.
type AgentRepository interface {
	FindByKID(ctx context.Context, kid []byte) (*model.Agent, error)
	FindByKIDIgnoreRevoked(ctx context.Context, kid []byte) (*model.Agent, error)
	Create(ctx context.Context, a *model.Agent) error
	RevokeByKID(ctx context.Context, kid []byte) error
	UnrevokeByKID(ctx context.Context, kid []byte) error
}

// SuitManifestRepository defines the interface for SUIT manifest persistence.
type SuitManifestRepository interface {
	FindLatestByTrustedComponentID(ctx context.Context, trustedComponentID []byte) (*model.SuitManifest, error)
	Create(ctx context.Context, m *model.SuitManifest) (int64, error)
}

// AgentHoldingSuitManifestRepository defines the interface for agent attributes and holding SUIT manifest persistence.
type AgentHoldingSuitManifestRepository interface {
	AddForAgent(ctx context.Context, agentID int64, suitManifestID int64) error
	ListActiveByAgent(ctx context.Context, agentID int64) ([]*model.AgentStatus, error)
}

// EntityRepository defines the interface for TC Developer persistence.
type EntityRepository interface {
	Create(ctx context.Context, dev *model.Entity) (int64, error)
	FindByID(ctx context.Context, id int64) (*model.Entity, error)
	FindByName(ctx context.Context, name string) (*model.Entity, error)
}

// ManifestSigningKeyRepository defines the interface for manifest signing key persistence.
type ManifestSigningKeyRepository interface {
	Create(ctx context.Context, key *model.ManifestSigningKey) (int64, error)
	FindByKID(ctx context.Context, kid []byte) (*model.ManifestSigningKey, error)
}

// TokenRepository defines the interface for token persistence.
type TokenRepository interface {
	Create(ctx context.Context, t *model.Token) (int64, error)
	FindByToken(ctx context.Context, tokenBytes []byte) (*model.Token, error)
	FindByID(ctx context.Context, id int64) (*model.Token, error)
	MarkConsumed(ctx context.Context, id int64) error
}

// ChallengeRepository defines the interface for challenge persistence.
type ChallengeRepository interface {
	Create(ctx context.Context, c *model.Challenge) (int64, error)
	FindByChallenge(ctx context.Context, challengeBytes []byte) (*model.Challenge, error)
	FindByID(ctx context.Context, id int64) (*model.Challenge, error)
	MarkConsumed(ctx context.Context, id int64) error
}

// SentQueryRequestMessageRepository defines the interface for sent query request message persistence.
type SentQueryRequestMessageRepository interface {
	Create(ctx context.Context, msg *model.SentQueryRequestMessage) (int64, error)
	FindByTokenID(ctx context.Context, tokenID int64) (*model.SentQueryRequestMessage, error)
	FindByChallengeID(ctx context.Context, challengeID int64) (*model.SentQueryRequestMessage, error)
	FindByID(ctx context.Context, id int64) (*model.SentQueryRequestMessage, error)
}
