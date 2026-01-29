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

func TestManifestSigningKey_CreateFindByKID_OK(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	// Create a TC Developer first
	entityRepo := NewEntityRepository(db)
	now := time.Now().UTC().Truncate(time.Second)
	dev := &model.Entity{Name: "Test Corp", IsTCDeveloper: true, CreatedAt: now}
	devID, err := entityRepo.Create(ctx, dev)
	if err != nil {
		t.Fatalf("Create developer error: %v", err)
	}

	// Create a manifest signing key
	keyRepo := NewManifestSigningKeyRepository(db)
	key := &model.ManifestSigningKey{
		KID:       []byte("key-1"),
		EntityID:  devID,
		PublicKey: []byte("pub-key-1"),
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
	}

	id, err := keyRepo.Create(ctx, key)
	if err != nil {
		t.Fatalf("Create key error: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}

	got, err := keyRepo.FindByKID(ctx, key.KID)
	if err != nil {
		t.Fatalf("FindByKID error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected key, got nil")
	}
	if got.EntityID != devID {
		t.Fatalf("developer id mismatch: want %d got %d", devID, got.EntityID)
	}
}

func TestManifestSigningKey_FindByKID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	keyRepo := NewManifestSigningKeyRepository(db)

	got, err := keyRepo.FindByKID(ctx, []byte("missing"))
	if err != nil {
		t.Fatalf("FindByKID error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
