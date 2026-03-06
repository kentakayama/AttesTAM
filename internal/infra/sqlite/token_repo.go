/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"

	"github.com/kentakayama/AttesTAM/internal/domain/model"
)

// TokenRepository handles token persistence.
type TokenRepository struct {
	db *sql.DB
}

func NewTokenRepository(db *sql.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

// Create inserts a new token and returns the inserted id.
func (r *TokenRepository) Create(ctx context.Context, t *model.Token) (int64, error) {
	const q = `
		INSERT INTO tokens (token, created_at, expired_at, consumed)
		VALUES (?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, q, t.Token, t.CreatedAt, t.ExpiredAt, t.Consumed)
	if err != nil {
		return 0, fmt.Errorf("insert token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindByToken returns a token by its token bytes.
func (r *TokenRepository) FindByToken(ctx context.Context, tokenBytes []byte) (*model.Token, error) {
	const q = `
		SELECT id, token, created_at, expired_at, consumed
		FROM tokens
		WHERE token = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, tokenBytes)
	var t model.Token
	if err := row.Scan(&t.ID, &t.Token, &t.CreatedAt, &t.ExpiredAt, &t.Consumed); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan token: %w", err)
	}
	return &t, nil
}

// MarkConsumed marks a token as consumed if it is not already consumed.
func (r *TokenRepository) MarkConsumed(ctx context.Context, token []byte) error {
	const q = `
		UPDATE tokens
		SET consumed = 1
		WHERE token = ? AND consumed = 0
	`
	res, err := r.db.ExecContext(ctx, q, token)
	if err != nil {
		return fmt.Errorf("update token: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("token already consumed or not found")
	}
	return nil
}

// FindByID returns a token by its ID. Basically not used.
func (r *TokenRepository) FindByID(ctx context.Context, id int64) (*model.Token, error) {
	const q = `
		SELECT id, token, created_at, expired_at, consumed
		FROM tokens
		WHERE id = ?
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, q, id)
	var t model.Token
	if err := row.Scan(&t.ID, &t.Token, &t.CreatedAt, &t.ExpiredAt, &t.Consumed); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan token: %w", err)
	}
	return &t, nil
}

// GenerateUniqueToken generates a unique token bytes using crypto/rand and ensures uniqueness by checking the database.
func (r *TokenRepository) GenerateUniqueToken(ctx context.Context) ([]byte, error) {
	const tokenSize = 32 // 256 bits
	for {
		token := make([]byte, tokenSize)
		if _, err := rand.Read(token); err != nil {
			return nil, fmt.Errorf("generate random token: %w", err)
		}
		// Check if it already exists
		existing, err := r.FindByToken(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("check token uniqueness: %w", err)
		}
		if existing == nil {
			c := model.Token{
				Token: token,
			}
			_, err := r.Create(ctx, &c)
			if err != nil {
				return nil, fmt.Errorf("insert new token: %w", err)
			}
			return token, nil
		}
		// If exists, try again
	}
}
