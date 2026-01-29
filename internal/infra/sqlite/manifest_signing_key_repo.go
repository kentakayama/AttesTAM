/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

// ManifestSigningKeyRepository handles manifest signing key persistence.
type ManifestSigningKeyRepository struct {
	db *sql.DB
}

func NewManifestSigningKeyRepository(db *sql.DB) *ManifestSigningKeyRepository {
	return &ManifestSigningKeyRepository{db: db}
}

// Create inserts a new manifest signing key and returns the inserted id.
func (r *ManifestSigningKeyRepository) Create(ctx context.Context, key *model.ManifestSigningKey) (int64, error) {
	const q = `
		INSERT INTO manifest_signing_keys (kid, entity_id, public_key, created_at, expired_at)
		VALUES (?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q, key.KID, key.EntityID, key.PublicKey, key.CreatedAt, key.ExpiredAt)
	if err != nil {
		return 0, fmt.Errorf("insert manifest_signing_key: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindByKID returns a manifest signing key by KID.
func (r *ManifestSigningKeyRepository) FindByKID(ctx context.Context, kid []byte) (*model.ManifestSigningKey, error) {
	const q = `
		SELECT id, kid, entity_id, public_key, created_at, expired_at
		FROM manifest_signing_keys
		WHERE kid = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, kid)
	var key model.ManifestSigningKey
	if err := row.Scan(&key.ID, &key.KID, &key.EntityID, &key.PublicKey, &key.CreatedAt, &key.ExpiredAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan manifest_signing_key: %w", err)
	}
	return &key, nil
}
