/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"errors"

	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/infra/sqlite"
	"github.com/kentakayama/tam-over-http/internal/suit"
)

// Insert SUIT_Envelope to the DB, while checking that:
// - if the same ComponentID exists,
//   - sequenceNumber is bigger than the existing one
//   - they are signed with the same TC Developer
//
// may accessed from outside the TAM, such as management API handler
func (t *TAM) SetEnvelope(envelopeBytes []byte, digest []byte, kid []byte, componentID []byte, sequenceNumber uint64) error {
	if envelopeBytes == nil {
		return errors.New("manifest is nil")
	}

	// check: does the TC Developer certainly exist?
	drepo := sqlite.NewManifestSigningKeyRepository(t.db)
	key, err := drepo.FindByKID(t.ctx, kid)
	if err != nil || key == nil {
		return ErrNotAuthenticated
	}

	mrepo := sqlite.NewSuitManifestRepository(t.db)

	existingManifest, err := t.GetManifest(componentID)
	if err != nil {
		return ErrFatal
	}
	if existingManifest != nil {
		// check: if exising manifest exists for the same ComponentID,
		// the sequenceNumber and kid are valid?
		if existingManifest.SequenceNumber >= sequenceNumber {
			return suit.ErrSUITManifestSmallerSequenceNumber
		}
		if existingManifest.SigningKeyID != key.ID {
			return suit.ErrSUITManifestSigningKeyMismatch
		}
	}

	manifest := &model.SuitManifest{
		Manifest:           envelopeBytes,
		Digest:             digest,
		SigningKeyID:       key.ID,
		TrustedComponentID: componentID,
		SequenceNumber:     sequenceNumber,
	}

	if _, err := mrepo.Create(t.ctx, manifest); err != nil {
		return err
	}

	return nil
}

// may accessed from outside the TAM, such as management API handler
func (t *TAM) GetManifest(componentID []byte) (*model.SuitManifest, error) {
	mrepo := sqlite.NewSuitManifestRepository(t.db)
	manifest, err := mrepo.FindLatestByTrustedComponentID(t.ctx, componentID)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}
