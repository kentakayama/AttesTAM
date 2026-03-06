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

func TestChallenge_CreateFindByChallenge(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewChallengeRepository(db)
	now := time.Now().UTC().Truncate(time.Second)

	challenge := &model.Challenge{
		Challenge: []byte("challenge-1"),
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		Consumed:  false,
	}

	id, err := repo.Create(ctx, challenge)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}

	got, err := repo.FindByChallenge(ctx, challenge.Challenge)
	if err != nil {
		t.Fatalf("FindByChallenge error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected challenge, got nil")
	}
	if got.Consumed {
		t.Fatalf("expected consumed=false, got true")
	}
}

func TestChallenge_MarkConsumed(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewChallengeRepository(db)
	now := time.Now().UTC().Truncate(time.Second)

	challenge := &model.Challenge{
		Challenge: []byte("challenge-2"),
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
		Consumed:  false,
	}

	id, err := repo.Create(ctx, challenge)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if err := repo.MarkConsumed(ctx, challenge.Challenge); err != nil {
		t.Fatalf("MarkConsumed error: %v", err)
	}

	got, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected challenge, got nil")
	}
	if !got.Consumed {
		t.Fatalf("expected consumed=true, got false")
	}
}

func TestChallenge_FindByChallenge_NotFound(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewChallengeRepository(db)

	got, err := repo.FindByChallenge(ctx, []byte("missing"))
	if err != nil {
		t.Fatalf("FindByChallenge error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
