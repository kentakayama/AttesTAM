/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kentakayama/tam-over-http/internal/domain"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

type DeviceRepository struct {
	db *sql.DB
}

// NewDeviceRepository creates a new instance of DeviceRepository.
func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) FindByUEID(ctx context.Context, ueid []byte) (*model.Device, error) {
	const query = `
		SELECT id, ueid, admin_id, created_at
		FROM devices
		WHERE ueid = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, ueid)
	var d model.Device
	if err := row.Scan(&d.ID, &d.UEID, &d.AdminID, &d.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	return &d, nil
}

func (r *DeviceRepository) Create(ctx context.Context, d *model.Device) (int64, error) {
	const query = `
		INSERT INTO devices (ueid, admin_id, created_at)
		VALUES (?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query, d.UEID, d.AdminID, d.CreatedAt)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return id, err
}
