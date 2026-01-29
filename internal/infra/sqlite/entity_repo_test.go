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

func TestEntity_CreateFindByID_OK(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewEntityRepository(db)
	now := time.Now().UTC().Truncate(time.Second)

	dev := &model.Entity{
		Name:      "Acme Corp",
		CreatedAt: now,
	}

	id, err := repo.Create(ctx, dev)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}

	got, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected developer, got nil")
	}
	if got.Name != dev.Name {
		t.Fatalf("name mismatch: want %q got %q", dev.Name, got.Name)
	}
}

func TestEntity_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewEntityRepository(db)

	got, err := repo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestEntity_FindByName_OK(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewEntityRepository(db)
	now := time.Now().UTC().Truncate(time.Second)

	dev := &model.Entity{
		Name:      "TestCorp",
		CreatedAt: now,
	}

	id, err := repo.Create(ctx, dev)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := repo.FindByName(ctx, "TestCorp")
	if err != nil {
		t.Fatalf("FindByName error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected developer, got nil")
	}
	if got.ID != id {
		t.Fatalf("id mismatch: want %d got %d", id, got.ID)
	}
	if got.Name != "TestCorp" {
		t.Fatalf("name mismatch: want TestCorp got %q", got.Name)
	}
}

func TestEntity_FindByName_NotFound(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	repo := NewEntityRepository(db)

	got, err := repo.FindByName(ctx, "NonExistent")
	if err != nil {
		t.Fatalf("FindByName error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
