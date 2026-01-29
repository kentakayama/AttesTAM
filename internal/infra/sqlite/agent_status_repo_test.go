/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

func TestAgenStatus_AddHoldingManifest_GetStatus(t *testing.T) {
	ctx := context.Background()
	db, err := InitDB(ctx, ":memory:")
	if err != nil {
		t.Fatalf("InitDB error: %v", err)
	}
	defer CloseDB(db)
	now := time.Now().UTC().Truncate(time.Second)

	// Create a device manager admin
	entityRepo := NewEntityRepository(db)
	deviceAdmin := &model.Entity{Name: "Building A Admin", IsDeviceAdmin: true, CreatedAt: now}
	deviceAdminID, err := entityRepo.Create(ctx, deviceAdmin)
	if err != nil {
		t.Fatalf("Create device admin error: %v", err)
	}

	// Create a device directly and get its ID
	ueid := append([]byte{0x01}, []byte("building-dev-123")...) // dummy random UEID with 128-bit field and 1 byte prefix
	res, err := db.ExecContext(ctx, "INSERT INTO devices (ueid, admin_id) VALUES (?, ?)", ueid, deviceAdminID)
	if err != nil {
		t.Fatalf("insert device error: %v", err)
	}
	deviceID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("lastinsertid agent error: %v", err)
	}

	// create an agent directly and get its ID
	agentKID := []byte("agent-1")
	_, err = db.ExecContext(ctx, "INSERT INTO agents (kid, device_id, public_key, created_at, expired_at) VALUES (?, ?, ?, ?, ?)", agentKID, deviceID, []byte("pk"), now, now.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("insert agent error: %v", err)
	}

	// Create TC Developer
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

	// create two manifests with same trusted_component_id
	manifestRepo := NewSuitManifestRepository(db)
	trusted := []byte("tc-99")
	digestM1 := []byte("digest1")
	digestM2 := []byte("digest2")
	digestM3 := []byte("digest3")
	m1 := &model.SuitManifest{Manifest: []byte("m1"), Digest: digestM1, SigningKeyID: keyID, TrustedComponentID: trusted, SequenceNumber: 1, CreatedAt: now}
	m2 := &model.SuitManifest{Manifest: []byte("m2"), Digest: digestM2, SigningKeyID: keyID, TrustedComponentID: trusted, SequenceNumber: 2, CreatedAt: now.Add(1 * time.Minute)}
	m3 := &model.SuitManifest{Manifest: []byte("m3"), Digest: digestM3, SigningKeyID: keyID, TrustedComponentID: []byte("another-tc"), SequenceNumber: 1, CreatedAt: now}

	_, err = manifestRepo.Create(ctx, m1)
	if err != nil {
		t.Fatalf("create m1: %v", err)
	}
	_, err = manifestRepo.Create(ctx, m2)
	if err != nil {
		t.Fatalf("create m2: %v", err)
	}
	_, err = manifestRepo.Create(ctx, m3)
	if err != nil {
		t.Fatalf("create m3: %v", err)
	}

	hrepo := NewAgentStatusRepository(db)

	// add first manifest
	if err := hrepo.AddForAgent(ctx, agentKID, digestM1); err != nil {
		t.Fatalf("AddForAgent m1 error: %v", err)
	}

	agentStatus1, err := hrepo.GetAgentStatus(ctx, agentKID)
	if err != nil {
		t.Fatalf("GetAgentStatus error: %v", err)
	}
	if !bytes.Equal(ueid, agentStatus1.DeviceUEID) {
		t.Fatalf("expected device ueid %s, got %s", hex.EncodeToString(ueid), hex.EncodeToString(agentStatus1.DeviceUEID))
	}
	if len(agentStatus1.SuitManifests) != 1 {
		t.Fatalf("expected 1 active holding, got %d", len(agentStatus1.SuitManifests))
	}
	if agentStatus1.SuitManifests[0].SequenceNumber != uint64(1) {
		t.Fatalf("expected suit manifest seq %d got %d", 1, agentStatus1.SuitManifests[0].SequenceNumber)
	}

	// add second manifest (same trusted component) -> first should be logically deleted
	if err := hrepo.AddForAgent(ctx, agentKID, digestM2); err != nil {
		t.Fatalf("AddForAgent m2 error: %v", err)
	}

	agentStatus2, err := hrepo.GetAgentStatus(ctx, agentKID)
	if err != nil {
		t.Fatalf("GetAgentStatus error: %v", err)
	}
	if len(agentStatus2.SuitManifests) != 1 {
		t.Fatalf("expected 1 active holding, got %d", len(agentStatus1.SuitManifests))
	}
	if agentStatus2.SuitManifests[0].SequenceNumber != uint64(2) {
		t.Fatalf("expected suit manifest seq %d got %d", 1, agentStatus2.SuitManifests[0].SequenceNumber)
	}

	// add second manifest (same trusted component) -> first should be logically deleted
	if err := hrepo.AddForAgent(ctx, agentKID, digestM2); err != nil {
		t.Fatalf("AddForAgent m2 error: %v", err)
	}

	// add third manifest (different trusted component)
	if err := hrepo.AddForAgent(ctx, agentKID, digestM3); err != nil {
		t.Fatalf("AddForAgent m2 error: %v", err)
	}

	agentStatus3, err := hrepo.GetAgentStatus(ctx, agentKID)
	if err != nil {
		t.Fatalf("GetAgentStatus error: %v", err)
	}
	if len(agentStatus3.SuitManifests) != 2 {
		t.Fatalf("expected 2 active holding, got %d", len(agentStatus3.SuitManifests))
	}
	if agentStatus3.SuitManifests[0].SequenceNumber != uint64(1) {
		t.Fatalf("expected suit manifest seq %d got %d", 1, agentStatus3.SuitManifests[0].SequenceNumber)
	}
	if agentStatus3.SuitManifests[1].SequenceNumber != uint64(2) {
		t.Fatalf("expected suit manifest seq %d got %d", 2, agentStatus3.SuitManifests[1].SequenceNumber)
	}
}
