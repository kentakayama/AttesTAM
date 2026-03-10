/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/kentakayama/AttesTAM/internal/domain/model"
)

// SuitManifestRepository handles SUIT manifest persistence.
type SuitManifestRepository struct {
	db *sql.DB
}

func NewSuitManifestRepository(db *sql.DB) *SuitManifestRepository {
	return &SuitManifestRepository{db: db}
}

func (r *SuitManifestRepository) FindByID(ctx context.Context, id int64) (*model.SuitManifest, error) {
	const q = `
		SELECT id, manifest, signing_key_id, trusted_component_id, sequence_number, created_at
		FROM suit_manifests
		WHERE id = ?
		ORDER BY sequence_number DESC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, id)
	var m model.SuitManifest
	if err := row.Scan(&m.ID, &m.Manifest, &m.SigningKeyID, &m.TrustedComponentID, &m.SequenceNumber, &m.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("suit manifest scan: %w", err)
	}
	return &m, nil
}

// FindLatestByTrustedComponentID returns the manifest with the largest sequence_number for a trusted component.
func (r *SuitManifestRepository) FindLatestByTrustedComponentID(ctx context.Context, trustedComponentID []byte) (*model.SuitManifest, error) {
	const q = `
		SELECT id, manifest, signing_key_id, trusted_component_id, sequence_number, created_at
		FROM suit_manifests
		WHERE trusted_component_id = ?
		ORDER BY sequence_number DESC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, trustedComponentID)
	var m model.SuitManifest
	if err := row.Scan(&m.ID, &m.Manifest, &m.SigningKeyID, &m.TrustedComponentID, &m.SequenceNumber, &m.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("suit manifest scan: %w", err)
	}
	return &m, nil
}

// FindLatestAll returns the latest manifest for each trusted component.
func (r *SuitManifestRepository) FindLatestAll(ctx context.Context) ([]model.SuitManifest, error) {
	const q = `
		SELECT sm.id, sm.manifest, sm.digest, sm.signing_key_id, sm.trusted_component_id, sm.sequence_number, sm.created_at
		FROM suit_manifests sm
		INNER JOIN (
			SELECT trusted_component_id, MAX(sequence_number) AS max_sequence_number
			FROM suit_manifests
			GROUP BY trusted_component_id
		) latest
			ON sm.trusted_component_id = latest.trusted_component_id
			AND sm.sequence_number = latest.max_sequence_number
		ORDER BY sm.trusted_component_id
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query latest suit manifests: %w", err)
	}
	defer rows.Close()

	manifests := make([]model.SuitManifest, 0)
	for rows.Next() {
		var m model.SuitManifest
		if err := rows.Scan(&m.ID, &m.Manifest, &m.Digest, &m.SigningKeyID, &m.TrustedComponentID, &m.SequenceNumber, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan latest suit manifest: %w", err)
		}
		manifests = append(manifests, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest suit manifests: %w", err)
	}

	return manifests, nil
}

// Create inserts a new SUIT manifest and returns the inserted id.
func (r *SuitManifestRepository) Create(ctx context.Context, m *model.SuitManifest) (int64, error) {
	if m.SequenceNumber >= math.MaxInt64 {
		return 0, errors.New("sequence-number exceeds the limit")
	}
	const q = `
		INSERT INTO suit_manifests (manifest, digest, signing_key_id, trusted_component_id, sequence_number, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q, m.Manifest, m.Digest, m.SigningKeyID, m.TrustedComponentID, m.SequenceNumber, m.CreatedAt)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}
