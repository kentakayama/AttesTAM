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

func TestSuitManifest_CreateFindLatest_OK(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	now := time.Now().UTC().Truncate(time.Second)

	// Create TC Developer
	entityRepo := NewEntityRepository(db)
	dev := &model.Entity{Name: "Test Corp", IsTCDeveloper: true, CreatedAt: now}
	devID, err := entityRepo.Create(ctx, dev)
	if err != nil {
		t.Fatalf("Create developer error: %v", err)
	}

	// Create manifest signing key
	keyRepo := NewManifestSigningKeyRepository(db)
	key := &model.ManifestSigningKey{
		KID:       []byte("key-1"),
		EntityID:  devID,
		PublicKey: []byte("pub-key-1"),
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
	}
	keyID, err := keyRepo.Create(ctx, key)
	if err != nil {
		t.Fatalf("Create key error: %v", err)
	}

	// Now create SUIT manifests
	repo := NewSuitManifestRepository(db)
	trusted := []byte("tc-1")

	m1 := &model.SuitManifest{
		Manifest:           []byte("mfst-1"),
		Digest:             []byte("digest-1"),
		SigningKeyID:       keyID,
		TrustedComponentID: trusted,
		SequenceNumber:     1,
		CreatedAt:          now,
	}
	m2 := &model.SuitManifest{
		Manifest:           []byte("mfst-2"),
		Digest:             []byte("digest-2"),
		SigningKeyID:       keyID,
		TrustedComponentID: trusted,
		SequenceNumber:     2,
		CreatedAt:          now.Add(1 * time.Minute),
	}

	id1, err := repo.Create(ctx, m1)
	if err != nil {
		t.Fatalf("Create m1 error: %v", err)
	}
	m1.ID = id1

	id2, err := repo.Create(ctx, m2)
	if err != nil {
		t.Fatalf("Create m2 error: %v", err)
	}
	m2.ID = id2

	got, err := repo.FindLatestByTrustedComponentID(ctx, trusted)
	if err != nil {
		t.Fatalf("FindLatestByTrustedComponentID error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected manifest, got nil")
	}
	if got.ID != m2.ID {
		t.Fatalf("expected latest id %d got %d", m2.ID, got.ID)
	}
	if got.SequenceNumber != m2.SequenceNumber {
		t.Fatalf("sequence mismatch: want %d got %d", m2.SequenceNumber, got.SequenceNumber)
	}
}
