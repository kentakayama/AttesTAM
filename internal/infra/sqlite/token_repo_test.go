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

	"github.com/kentakayama/AttesTAM/internal/domain/model"
)

func TestToken_CreateFindByToken(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewTokenRepository(db)
	now := time.Now().UTC().Truncate(time.Second)

	token := &model.Token{
		Token:     []byte("token-1"),
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		Consumed:  false,
	}

	id, err := repo.Create(ctx, token)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}

	got, err := repo.FindByToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("FindByToken error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected token, got nil")
	}
	if got.Consumed {
		t.Fatalf("expected consumed=false, got true")
	}
}

func TestToken_MarkConsumed(t *testing.T) {
	tokenBytes := []byte("token-2")
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewTokenRepository(db)
	now := time.Now().UTC().Truncate(time.Second)

	token := &model.Token{
		Token:     tokenBytes,
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		Consumed:  false,
	}

	id, err := repo.Create(ctx, token)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if err := repo.MarkConsumed(ctx, tokenBytes); err != nil {
		t.Fatalf("MarkConsumed error: %v", err)
	}

	got, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected token, got nil")
	}
	if !got.Consumed {
		t.Fatalf("expected consumed=true, got false")
	}
}

func TestToken_FindByToken_NotFound(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewTokenRepository(db)

	got, err := repo.FindByToken(ctx, []byte("missing"))
	if err != nil {
		t.Fatalf("FindByToken error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
