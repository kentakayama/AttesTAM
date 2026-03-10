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

func TestSuitManifest_FindLatestAll_OK(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)

	now := time.Now().UTC().Truncate(time.Second)

	entityRepo := NewEntityRepository(db)
	devID, err := entityRepo.Create(ctx, &model.Entity{Name: "Test Corp", IsTCDeveloper: true, CreatedAt: now})
	if err != nil {
		t.Fatalf("Create developer error: %v", err)
	}

	keyRepo := NewManifestSigningKeyRepository(db)
	keyID, err := keyRepo.Create(ctx, &model.ManifestSigningKey{
		KID:       []byte("key-1"),
		EntityID:  devID,
		PublicKey: []byte("pub-key-1"),
		CreatedAt: now,
		ExpiredAt: now.Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Create key error: %v", err)
	}

	repo := NewSuitManifestRepository(db)
	manifests := []*model.SuitManifest{
		{
			Manifest:           []byte("mfst-1"),
			Digest:             []byte("digest-1"),
			SigningKeyID:       keyID,
			TrustedComponentID: []byte("tc-1"),
			SequenceNumber:     1,
			CreatedAt:          now,
		},
		{
			Manifest:           []byte("mfst-2"),
			Digest:             []byte("digest-2"),
			SigningKeyID:       keyID,
			TrustedComponentID: []byte("tc-1"),
			SequenceNumber:     2,
			CreatedAt:          now.Add(1 * time.Minute),
		},
		{
			Manifest:           []byte("mfst-3"),
			Digest:             []byte("digest-3"),
			SigningKeyID:       keyID,
			TrustedComponentID: []byte("tc-2"),
			SequenceNumber:     7,
			CreatedAt:          now.Add(2 * time.Minute),
		},
	}

	for _, manifest := range manifests {
		if _, err := repo.Create(ctx, manifest); err != nil {
			t.Fatalf("Create manifest error: %v", err)
		}
	}

	got, err := repo.FindLatestAll(ctx)
	if err != nil {
		t.Fatalf("FindLatestAll error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(got))
	}
	if string(got[0].TrustedComponentID) != "tc-1" || got[0].SequenceNumber != 2 {
		t.Fatalf("unexpected first manifest: tc=%s seq=%d", got[0].TrustedComponentID, got[0].SequenceNumber)
	}
	if string(got[1].TrustedComponentID) != "tc-2" || got[1].SequenceNumber != 7 {
		t.Fatalf("unexpected second manifest: tc=%s seq=%d", got[1].TrustedComponentID, got[1].SequenceNumber)
	}
}
