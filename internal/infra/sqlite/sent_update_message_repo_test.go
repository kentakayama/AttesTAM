/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
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

func TestSentUpdateMessageRepository_FindWithManifestsByToken(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer CloseDB(db)

	// Insert dummy data for foreign keys
	_, err = db.ExecContext(ctx, "INSERT INTO agents (kid, public_key, created_at, expired_at) VALUES (?, ?, ?, ?)", []byte("kid1"), []byte("pubkey"), time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to insert agent: %v", err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO entities (name, is_tc_developer, created_at) VALUES (?, ?, ?)", "test dev", 1, time.Now())
	if err != nil {
		t.Fatalf("failed to insert tc developer: %v", err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO manifest_signing_keys (kid, entity_id, public_key, created_at, expired_at) VALUES (?, ?, ?, ?, ?)", []byte("kid2"), 1, []byte("pubkey2"), time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to insert manifest signing key: %v", err)
	}

	// Repositories
	tokenRepo := NewTokenRepository(db)
	suitManifestRepo := NewSuitManifestRepository(db)
	sentUpdateMessageRepo := NewSentUpdateMessageRepository(db)

	// Create a token
	token := &model.Token{
		Token:     []byte("test_token"),
		CreatedAt: time.Now(),
		ExpiredAt: time.Now().Add(time.Hour),
		Consumed:  false,
	}
	tokenID, err := tokenRepo.Create(ctx, token)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Create a SUIT manifest
	manifest := &model.SuitManifest{
		Manifest:           []byte("dummy manifest"),
		Digest:             []byte("dummy digest"),
		SigningKeyID:       1,
		TrustedComponentID: []byte("tc123"),
		SequenceNumber:     1,
		CreatedAt:          time.Now(),
	}
	manifestID, err := suitManifestRepo.Create(ctx, manifest)
	if err != nil {
		t.Fatalf("failed to create suit manifest: %v", err)
	}

	// Create a sent update message
	sentMsg := &model.SentUpdateMessage{
		AgentID:   1,
		TokenID:   tokenID,
		CreatedAt: time.Now(),
	}
	sentMsgID, err := sentUpdateMessageRepo.Create(ctx, sentMsg)
	if err != nil {
		t.Fatalf("failed to create sent update message: %v", err)
	}

	// Insert into sent_manifests_in_update_messages
	const insertManifestQuery = `
		INSERT INTO sent_manifests_in_update_messages (sent_update_id, suit_manifest_id)
		VALUES (?, ?)
	`
	_, err = db.ExecContext(ctx, insertManifestQuery, sentMsgID, manifestID)
	if err != nil {
		t.Fatalf("failed to insert into sent_manifests_in_update_messages: %v", err)
	}

	// Find with manifests by token
	result, err := sentUpdateMessageRepo.FindWithManifestsByToken(ctx, []byte("test_token"))
	if err != nil {
		t.Fatalf("failed to find with manifests by token: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Verify token
	if result.Token.ID != tokenID {
		t.Errorf("expected token ID %d, got %d", tokenID, result.Token.ID)
	}
	if string(result.Token.Token) != "test_token" {
		t.Errorf("expected token %s, got %s", "test_token", string(result.Token.Token))
	}

	// Verify sent update message
	if result.SentUpdateMessage.ID != sentMsgID {
		t.Errorf("expected sent update message ID %d, got %d", sentMsgID, result.SentUpdateMessage.ID)
	}
	if result.SentUpdateMessage.TokenID != tokenID {
		t.Errorf("expected token ID %d, got %d", tokenID, result.SentUpdateMessage.TokenID)
	}

	// Verify manifests
	if len(result.Manifests) != 1 {
		t.Errorf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].ID != manifestID {
		t.Errorf("expected manifest ID %d, got %d", manifestID, result.Manifests[0].ID)
	}
	if string(result.Manifests[0].Manifest) != "dummy manifest" {
		t.Errorf("expected manifest %s, got %s", "dummy manifest", string(result.Manifests[0].Manifest))
	}
}
