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

// EntityRepository handles Trusted Component Developer persistence.
type EntityRepository struct {
	db *sql.DB
}

func NewEntityRepository(db *sql.DB) *EntityRepository {
	return &EntityRepository{db: db}
}

// Create inserts a new TC Developer and returns the inserted id.
func (r *EntityRepository) Create(ctx context.Context, dev *model.Entity) (int64, error) {
	const q = `
		INSERT INTO entities (name, is_tam_admin, is_tc_developer, is_device_admin, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q, dev.Name, dev.IsTAMAdmin, dev.IsTCDeveloper, dev.IsDeviceAdmin, dev.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert tc_developer: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindByID returns a TC Developer by ID. Basically not used.
func (r *EntityRepository) FindByID(ctx context.Context, id int64) (*model.Entity, error) {
	const q = `
		SELECT id, name, created_at
		FROM entities
		WHERE id = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, id)
	var dev model.Entity
	if err := row.Scan(&dev.ID, &dev.Name, &dev.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan entity: %w", err)
	}
	return &dev, nil
}

// FindByName returns a TC Developer by name.
func (r *EntityRepository) FindByName(ctx context.Context, name string) (*model.Entity, error) {
	const q = `
		SELECT id, name, created_at
		FROM entities
		WHERE name = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, name)
	var dev model.Entity
	if err := row.Scan(&dev.ID, &dev.Name, &dev.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan tc_developer: %w", err)
	}
	return &dev, nil
}
