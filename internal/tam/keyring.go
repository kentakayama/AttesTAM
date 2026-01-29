/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"crypto"
	"errors"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/infra/sqlite"
	cose "github.com/veraison/go-cose"
)

func (t *TAM) setTEEPAgentKey(key *cose.Key, ueid []byte) error {
	if key == nil {
		return errors.New("public key is nil")
	}

	kid, err := key.Thumbprint(crypto.SHA256)
	if err != nil || len(kid) != 32 {
		return nil
	}
	pubKeyBytes, err := cbor.Marshal(key)
	if err != nil {
		return err
	}

	arepo := sqlite.NewAgentRepository(t.db)

	// Check if the key already exists (ignoring revoked status)
	existing, err := arepo.FindByKIDIgnoreRevoked(t.ctx, kid)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	// look up device
	var deviceID *int64
	if ueid != nil {
		drepo := sqlite.NewDeviceRepository(t.db)
		device, _ := drepo.FindByUEID(t.ctx, ueid)
		// ignore errors here
		if device != nil {
			deviceID = &device.ID
		}
	}

	// Create new agent key
	now := time.Now().UTC().Truncate(time.Second)
	agent := &model.Agent{
		KID:       kid,
		DeviceID:  deviceID,
		PublicKey: pubKeyBytes,
		CreatedAt: now,
		ExpiredAt: now.Add(365 * 24 * time.Hour), // Valid for 1 year
	}

	if _, err := arepo.Create(t.ctx, agent); err != nil {
		return err
	}

	return nil
}

func (t *TAM) getTEEPAgentKey(kid []byte) (*cose.Key, error) {
	if len(kid) != 32 {
		return nil, errors.New("invalid key length (expected: 32)")
	}

	arepo := sqlite.NewAgentRepository(t.db)
	key, err := arepo.FindByKID(t.ctx, kid)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}

	var coseKey cose.Key
	if err := cbor.Unmarshal(key.PublicKey, &coseKey); err != nil {
		return nil, err
	}
	return &coseKey, nil
}

func (t *TAM) revokeTEEPAgentKey(kid []byte) error {
	if len(kid) != 32 {
		return errors.New("invalid key length (expected: 32)")
	}
	arepo := sqlite.NewAgentRepository(t.db)
	err := arepo.RevokeByKID(t.ctx, kid)
	if err != nil {
		return err
	}
	return nil
}

// may accessed from outside the TAM, such as management API handler
func (t *TAM) GetEntityKey(kid []byte) (*cose.Key, error) {
	if len(kid) != 32 {
		return nil, errors.New("invalid key length (expected: 32)")
	}

	mrepo := sqlite.NewManifestSigningKeyRepository(t.db)
	key, err := mrepo.FindByKID(t.ctx, kid)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}

	var coseKey cose.Key
	if err := cbor.Unmarshal(key.PublicKey, &coseKey); err != nil {
		return nil, err
	}
	return &coseKey, nil
}
