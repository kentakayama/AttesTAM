/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kentakayama/tam-over-http/internal/domain"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

func TestSQLite_InitCreateFindClose_OK(t *testing.T) {
	ctx := context.Background()

	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewAgentRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	a := &model.Agent{
		KID:       []byte("kid-1"), // NOTE: Dummy bytes for testing only — replace with COSE Key Thumbprint in production.
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		PublicKey: []byte("pk-1"), // NOTE: COSE Key in production.
	}

	id, err := repo.Create(ctx, a)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := repo.FindByKID(ctx, a.KID)
	if err != nil {
		t.Fatalf("FindByKID error: %v", err)
	}

	if got.ID != id {
		t.Fatalf("ID mismatch: got %v want %v", got.ID, id)
	}
	if !bytes.Equal(got.KID, a.KID) {
		t.Fatalf("KID mismatch: got %v want %v", got.KID, a.KID)
	}
	if !bytes.Equal(got.PublicKey, a.PublicKey) {
		t.Fatalf("PublicKey mismatch: got %v want %v", got.PublicKey, a.PublicKey)
	}
	if !got.CreatedAt.Equal(a.CreatedAt) {
		t.Fatalf("CreatedAt mismatch: got %v want %v", got.CreatedAt, a.CreatedAt)
	}
	if !got.ExpiredAt.Equal(a.ExpiredAt) {
		t.Fatalf("ExpiredAt mismatch: got %v want %v", got.ExpiredAt, a.ExpiredAt)
	}

	if err := CloseDB(db); err != nil {
		t.Fatalf("CloseDB error: %v", err)
	}
}

func TestSQLite_FindByID_NotFound_And_Expired(t *testing.T) {
	ctx := context.Background()

	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewAgentRepository(db)

	// Not found
	_, err = repo.FindByKID(ctx, []byte("missing"))
	if err == nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}

	// Expired
	now := time.Now().UTC().Truncate(time.Second)
	expired := &model.Agent{
		KID:       []byte("kid-exp"),
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiredAt: now.Add(-1 * time.Hour),
		PublicKey: []byte("pk-exp"),
	}
	if _, err := repo.Create(ctx, expired); err != nil {
		t.Fatalf("Create expired agent error: %v", err)
	}

	_, err = repo.FindByKID(ctx, expired.KID)
	if err == nil || !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("expected ErrExpired, got: %v", err)
	}
}

func TestSQLite_RevokeByKID_And_FindByKID_Revoked(t *testing.T) {
	ctx := context.Background()

	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewAgentRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	agent := &model.Agent{
		KID:       []byte("kid-revoke"),
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		PublicKey: []byte("pk-revoke"),
	}

	id, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("Create agent error: %v", err)
	}

	// Verify agent can be found before revocation
	got, err := repo.FindByKID(ctx, agent.KID)
	if err != nil {
		t.Fatalf("FindByKID before revoke error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected agent, got nil")
	}
	if got.ID != id {
		t.Fatalf("ID mismatch: got %v want %v", got.ID, id)
	}

	// Revoke the agent
	if err := repo.RevokeByKID(ctx, agent.KID); err != nil {
		t.Fatalf("RevokeByKID error: %v", err)
	}

	// Verify agent cannot be found after revocation
	_, err = repo.FindByKID(ctx, agent.KID)
	if err == nil || !errors.Is(err, domain.ErrRevoked) {
		t.Fatalf("expected ErrRevoked, got: %v", err)
	}
}

func TestSQLite_RevokeByKID_NotFound(t *testing.T) {
	ctx := context.Background()

	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewAgentRepository(db)

	// Try to revoke non-existent agent
	err = repo.RevokeByKID(ctx, []byte("missing"))
	if err == nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}
