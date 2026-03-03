/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

func TestSentQueryRequestMessage_Pattern1_TokenWithoutChallenge(t *testing.T) {
	tokenBytes := []byte("token-test")

	// Pattern 1: token exists, no challenge, tc_list_requested=TRUE, attestation_requested=FALSE, agent_id exists
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	now := time.Now().UTC().Truncate(time.Second)

	// Create an agent
	agentRes, err := db.ExecContext(ctx, "INSERT INTO agents (kid, public_key, created_at, expired_at) VALUES (?, ?, ?, ?)", []byte("agent-test"), []byte("pk"), now, now.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("insert agent error: %v", err)
	}
	agentID, err := agentRes.LastInsertId()
	if err != nil {
		t.Fatalf("lastinsertid agent error: %v", err)
	}

	// Create a token
	tokenRepo := NewTokenRepository(db)
	token := &model.Token{
		Token:     tokenBytes,
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		Consumed:  false,
	}
	tokenID, err := tokenRepo.Create(ctx, token)
	if err != nil {
		t.Fatalf("Create token error: %v", err)
	}

	// Create sent query request message
	repo := NewSentQueryRequestMessageRepository(db)
	msg := &model.SentQueryRequestMessage{
		AgentID:              &agentID,
		AttestationRequested: false,
		TCListRequested:      true,
		TokenID:              &tokenID,
		ChallengeID:          nil,
		CreatedAt:            now,
	}

	msgID, err := repo.Create(ctx, msg)
	if err != nil {
		t.Fatalf("Create message error: %v", err)
	}

	// Find by token
	got, err := repo.FindByToken(ctx, tokenBytes)
	if err != nil {
		t.Fatalf("FindByTokenID error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected message, got nil")
	}
	if got.SentQueryRequestMessage.ID != msgID {
		t.Fatalf("message id mismatch: want %d got %d", msgID, got.SentQueryRequestMessage.ID)
	}
	if *got.SentQueryRequestMessage.AgentID != agentID {
		t.Fatalf("agent id mismatch: want %d got %d", agentID, *got.SentQueryRequestMessage.AgentID)
	}
	if got.SentQueryRequestMessage.AttestationRequested {
		t.Fatalf("expected attestation_requested=false, got true")
	}
	if !got.SentQueryRequestMessage.TCListRequested {
		t.Fatalf("expected tc_list_requested=true, got false")
	}
	if got.SentQueryRequestMessage.ChallengeID != nil {
		t.Fatalf("expected challenge_id=nil, got %d", *got.SentQueryRequestMessage.ChallengeID)
	}
}

func TestSentQueryRequestMessage_Pattern2_ChallengeWithoutToken(t *testing.T) {
	challengeBytes := []byte("challenge-test")

	// Pattern 2: no token, challenge exists, tc_list_requested=TRUE, attestation_requested=TRUE, no agent_id (NULL)
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	now := time.Now().UTC().Truncate(time.Second)

	// Create a challenge
	challengeRepo := NewChallengeRepository(db)
	challenge := &model.Challenge{
		Challenge: challengeBytes,
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		Consumed:  false,
	}
	challengeID, err := challengeRepo.Create(ctx, challenge)
	if err != nil {
		t.Fatalf("Create challenge error: %v", err)
	}

	// Create sent query request message (no agent_id)
	repo := NewSentQueryRequestMessageRepository(db)
	msg := &model.SentQueryRequestMessage{
		AgentID:              nil, // NULL
		AttestationRequested: true,
		TCListRequested:      true,
		TokenID:              nil,
		ChallengeID:          &challengeID,
		CreatedAt:            now,
	}

	msgID, err := repo.Create(ctx, msg)
	if err != nil {
		t.Fatalf("Create message error: %v", err)
	}

	// Find by challenge
	got, err := repo.FindByChallenge(ctx, challengeBytes)
	if err != nil {
		t.Fatalf("FindByChallengeID error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected message, got nil")
	}
	if got.SentQueryRequestMessage.ID != msgID {
		t.Fatalf("message id mismatch: want %d got %d", msgID, got.SentQueryRequestMessage.ID)
	}
	if got.SentQueryRequestMessage.AgentID != nil {
		t.Fatalf("expected agent_id=nil, got %d", *got.SentQueryRequestMessage.AgentID)
	}
	if !got.SentQueryRequestMessage.AttestationRequested {
		t.Fatalf("expected attestation_requested=true, got false")
	}
	if !got.SentQueryRequestMessage.TCListRequested {
		t.Fatalf("expected tc_list_requested=true, got false")
	}
	if got.SentQueryRequestMessage.TokenID != nil {
		t.Fatalf("expected token_id=nil, got %d", *got.SentQueryRequestMessage.TokenID)
	}
}

func TestSentQueryRequestMessage_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewSentQueryRequestMessageRepository(db)

	got, err := repo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
