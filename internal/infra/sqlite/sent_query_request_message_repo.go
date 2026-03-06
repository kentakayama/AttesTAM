/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kentakayama/AttesTAM/internal/domain/model"
)

// SentQueryRequestMessageWithToken represents a sent query request message with its associated token.
type SentQueryRequestMessageWithToken struct {
	SentQueryRequestMessage model.SentQueryRequestMessage
	Token                   model.Token
}

// SentQueryRequestMessageWithChallenge represents a sent query request message with its associated challenge.
type SentQueryRequestMessageWithChallenge struct {
	SentQueryRequestMessage model.SentQueryRequestMessage
	Challenge               model.Challenge
}

// SentQueryRequestMessageRepository handles sent query request message persistence.
type SentQueryRequestMessageRepository struct {
	db *sql.DB
}

func NewSentQueryRequestMessageRepository(db *sql.DB) *SentQueryRequestMessageRepository {
	return &SentQueryRequestMessageRepository{db: db}
}

// Create inserts a new sent query request message and returns the inserted id.
func (r *SentQueryRequestMessageRepository) Create(ctx context.Context, msg *model.SentQueryRequestMessage) (int64, error) {
	const q = `
		INSERT INTO sent_query_request_messages (agent_id, attestation_requested, tc_list_requested, token_id, challenge_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q, msg.AgentID, msg.AttestationRequested, msg.TCListRequested, msg.TokenID, msg.ChallengeID, msg.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert sent query request message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// CreateWithToken inserts a new sent query request message using a token bytes and returns the inserted id.
func (r *SentQueryRequestMessageRepository) CreateWithToken(ctx context.Context, token []byte, msg *model.SentQueryRequestMessage) (int64, error) {
	// Find the token by bytes
	tokenRepo := NewTokenRepository(r.db)
	tok, err := tokenRepo.FindByToken(ctx, token)
	if err != nil {
		return 0, fmt.Errorf("find token: %w", err)
	}
	if tok == nil {
		return 0, fmt.Errorf("token not found")
	}
	if tok.Consumed {
		return 0, fmt.Errorf("token consumed")
	}
	msg.TokenID = &tok.ID
	return r.Create(ctx, msg)
}

// CreateWithChallenge inserts a new sent query request message using a challenge bytes and returns the inserted id.
func (r *SentQueryRequestMessageRepository) CreateWithChallenge(ctx context.Context, challenge []byte, msg *model.SentQueryRequestMessage) (int64, error) {
	// Find the challenge by bytes
	challengeRepo := NewChallengeRepository(r.db)
	chal, err := challengeRepo.FindByChallenge(ctx, challenge)
	if err != nil {
		return 0, fmt.Errorf("find challenge: %w", err)
	}
	if chal == nil {
		return 0, fmt.Errorf("challenge not found")
	}
	if chal.Consumed {
		return 0, fmt.Errorf("challenge consumed")
	}
	msg.ChallengeID = &chal.ID
	return r.Create(ctx, msg)
}

// FindByToken returns sent query request messages by token.
func (r *SentQueryRequestMessageRepository) FindByToken(ctx context.Context, token []byte) (*SentQueryRequestMessageWithToken, error) {
	const q = `
		SELECT t.id, t.token, t.created_at, t.expired_at, t.consumed,
		       sqrm.id, sqrm.agent_id, sqrm.attestation_requested, sqrm.tc_list_requested, sqrm.token_id, sqrm.challenge_id, sqrm.created_at
		FROM tokens t
		JOIN sent_query_request_messages sqrm ON t.id = sqrm.token_id
		WHERE t.token = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, token)
	var tok model.Token
	var msg model.SentQueryRequestMessage
	var agentID sql.NullInt64
	var tokenID sql.NullInt64
	var challengeID sql.NullInt64

	err := row.Scan(
		&tok.ID, &tok.Token, &tok.CreatedAt, &tok.ExpiredAt, &tok.Consumed,
		&msg.ID, &agentID, &msg.AttestationRequested, &msg.TCListRequested, &tokenID, &challengeID, &msg.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan sent query request message with token: %w", err)
	}

	if agentID.Valid {
		msg.AgentID = &agentID.Int64
	}
	if tokenID.Valid {
		msg.TokenID = &tokenID.Int64
	}
	if challengeID.Valid {
		msg.ChallengeID = &challengeID.Int64
	}

	return &SentQueryRequestMessageWithToken{
		SentQueryRequestMessage: msg,
		Token:                   tok,
	}, nil
}

// FindByChallenge returns sent query request messages by challenge.
func (r *SentQueryRequestMessageRepository) FindByChallenge(ctx context.Context, challenge []byte) (*SentQueryRequestMessageWithChallenge, error) {
	const q = `
		SELECT c.id, c.challenge, c.created_at, c.expired_at, c.consumed,
		       sqrm.id, sqrm.agent_id, sqrm.attestation_requested, sqrm.tc_list_requested, sqrm.token_id, sqrm.challenge_id, sqrm.created_at
		FROM challenges c
		JOIN sent_query_request_messages sqrm ON c.id = sqrm.challenge_id
		WHERE c.challenge = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, challenge)
	var chal model.Challenge
	var msg model.SentQueryRequestMessage
	var agentID sql.NullInt64
	var tokenID sql.NullInt64
	var challengeID sql.NullInt64

	err := row.Scan(
		&chal.ID, &chal.Challenge, &chal.CreatedAt, &chal.ExpiredAt, &chal.Consumed,
		&msg.ID, &agentID, &msg.AttestationRequested, &msg.TCListRequested, &tokenID, &challengeID, &msg.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan sent query request message with challenge: %w", err)
	}

	if agentID.Valid {
		msg.AgentID = &agentID.Int64
	}
	if tokenID.Valid {
		msg.TokenID = &tokenID.Int64
	}
	if challengeID.Valid {
		msg.ChallengeID = &challengeID.Int64
	}

	return &SentQueryRequestMessageWithChallenge{
		SentQueryRequestMessage: msg,
		Challenge:               chal,
	}, nil
}

// FindByID returns a sent query request message by ID. Basically not used.
func (r *SentQueryRequestMessageRepository) FindByID(ctx context.Context, id int64) (*model.SentQueryRequestMessage, error) {
	const q = `
		SELECT id, agent_id, attestation_requested, tc_list_requested, token_id, challenge_id, created_at
		FROM sent_query_request_messages
		WHERE id = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, id)
	var msg model.SentQueryRequestMessage
	if err := row.Scan(&msg.ID, &msg.AgentID, &msg.AttestationRequested, &msg.TCListRequested, &msg.TokenID, &msg.ChallengeID, &msg.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan sent query request message: %w", err)
	}
	return &msg, nil
}
