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

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes the SQLite database and creates necessary tables.
func InitDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Connection-level pragmas to improve concurrency and reliability.
	// These are executed per-connection; setting them here ensures sensible defaults.
	// NOTE: Some pragmas are persistent per DB file (journal_mode) and return a row.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set PRAGMA foreign_keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set PRAGMA journal_mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = NORMAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set PRAGMA synchronous: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set PRAGMA busy_timeout: %w", err)
	}

	// Create tables and indexes
	if err := createSchema(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return db, nil
}

// createSchema creates all necessary database tables.
func createSchema(ctx context.Context, db *sql.DB) error {
	schema := `
	-- Enable foreign keys
	PRAGMA foreign_keys = ON;

	-- TC Developers table
	CREATE TABLE IF NOT EXISTS entities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		is_tam_admin INTEGER DEFAULT 0,
		is_tc_developer INTEGER DEFAULT 0,
		is_device_admin INTEGER DEFAULT 0,
		-- TODO: credential
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Create index on name for faster lookups
	CREATE INDEX IF NOT EXISTS idx_entity_name ON entities(name);

	-- Manifest Signing Keys for TC Developers table
	CREATE TABLE IF NOT EXISTS manifest_signing_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kid BLOB UNIQUE NOT NULL,
		entity_id INTEGER NOT NULL,
		public_key BLOB NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expired_at TIMESTAMP NOT NULL,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
	);

	-- Create index on kid for faster lookups
	CREATE INDEX IF NOT EXISTS idx_manifest_signing_keys_kid ON manifest_signing_keys(kid);
	CREATE INDEX IF NOT EXISTS idx_manifest_signing_keys_expired_at ON manifest_signing_keys(expired_at);

	-- SUIT Manifests table
	CREATE TABLE IF NOT EXISTS suit_manifests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		manifest BLOB NOT NULL,
		digest BLOB NOT NULL,
		signing_key_id INTEGER NOT NULL,
		trusted_component_id BLOB NOT NULL,
		sequence_number INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (signing_key_id) REFERENCES manifest_signing_keys(id) ON DELETE CASCADE
	);

	-- Create index on trusted_component_id for faster lookups
	CREATE INDEX IF NOT EXISTS idx_suit_manifests_digest ON suit_manifests(digest);
	CREATE INDEX IF NOT EXISTS idx_suit_manifests_trusted_component_id ON suit_manifests(trusted_component_id);
	CREATE INDEX IF NOT EXISTS idx_suit_manifests_sequence_number ON suit_manifests(sequence_number);

	-- Composite index to accelerate "find latest by trusted_component_id ORDER BY sequence_number DESC"
	CREATE INDEX IF NOT EXISTS idx_suit_manifests_tc_seq ON suit_manifests(trusted_component_id, sequence_number);

	-- Devices table
	CREATE TABLE IF NOT EXISTS devices (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ueid BLOB UNIQUE NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		admin_id INTEGER,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (admin_id) REFERENCES entities(id) ON DELETE CASCADE
	);

	-- Agents table
	CREATE TABLE IF NOT EXISTS agents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kid BLOB UNIQUE NOT NULL,
		device_id INTEGER NULLABLE,
		public_key BLOB NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expired_at TIMESTAMP NOT NULL,
		revoked_at INTEGER,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	-- Create index on kid for faster lookups
	CREATE INDEX IF NOT EXISTS idx_agents_kid ON agents(kid);
	CREATE INDEX IF NOT EXISTS idx_agents_expired_at ON agents(expired_at);
	CREATE INDEX IF NOT EXISTS idx_agents_revoked_at ON agents(revoked_at);

	-- Agent Holding SUIT Manifest table
	CREATE TABLE IF NOT EXISTS agent_holding_suit_manifests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		suit_manifest_id INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted BOOLEAN NOT NULL DEFAULT 0,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (suit_manifest_id) REFERENCES suit_manifests(id) ON DELETE CASCADE,
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
	);

	-- Create index on agent_id, deleted and suit_manifest_id for faster lookups
	-- Expecting queries filtering by agent_id and NOT deleted status and
	-- ones filtering by suit_manifest_id
	CREATE INDEX IF NOT EXISTS idx_agent_holding_suit_manifests_agent_id_deleted ON agent_holding_suit_manifests(agent_id, deleted);
	CREATE INDEX IF NOT EXISTS idx_agent_holding_suit_manifests_suit_manifest_id ON agent_holding_suit_manifests(suit_manifest_id);

	-- Tokens table
	CREATE TABLE IF NOT EXISTS tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token BLOB UNIQUE NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expired_at TIMESTAMP NOT NULL,
		consumed BOOLEAN NOT NULL DEFAULT 0
	);

	-- Create index on token for faster lookups
	CREATE INDEX IF NOT EXISTS idx_tokens_token ON tokens(token);

	-- Challenges table
	CREATE TABLE IF NOT EXISTS challenges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		challenge BLOB UNIQUE NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expired_at TIMESTAMP NOT NULL,
		consumed BOOLEAN NOT NULL DEFAULT 0
	);
	
	-- Create index on challenge for faster lookups
	CREATE INDEX IF NOT EXISTS idx_challenges_challenge ON challenges(challenge);

	-- Sent QueryRequests table
	CREATE TABLE IF NOT EXISTS sent_query_request_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NULLABLE, -- to allow messages without associated agents
		attestation_requested BOOLEAN NOT NULL,
		tc_list_requested BOOLEAN NOT NULL,
		token_id INTEGER NULLABLE, -- if challenge is used, token can be null
		challenge_id INTEGER NULLABLE, -- if token is used, challenge can be null
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
		FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE SET NULL,
		FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE SET NULL
	);

	-- Sent QueryRequest messages indexes
	CREATE INDEX IF NOT EXISTS idx_sent_qr_token_id ON sent_query_request_messages(token_id);
	CREATE INDEX IF NOT EXISTS idx_sent_qr_challenge_id ON sent_query_request_messages(challenge_id);

	-- Index for manifest signing key lookup
	CREATE INDEX IF NOT EXISTS idx_suit_manifests_signing_key_id ON suit_manifests(signing_key_id);

	-- Sent Updates table
	CREATE TABLE IF NOT EXISTS sent_update_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		token_id INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
		FOREIGN KEY (token_id) REFERENCES tokens(id) ON DELETE CASCADE
	);

	-- SUIT Manifests Sent in Updates table
	CREATE TABLE IF NOT EXISTS sent_manifests_in_update_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sent_update_id INTEGER NOT NULL,
		suit_manifest_id INTEGER NOT NULL,
		-- table constraints (placed after column definitions for compatibility)
		FOREIGN KEY (sent_update_id) REFERENCES sent_update_messages(id) ON DELETE CASCADE,
		FOREIGN KEY (suit_manifest_id) REFERENCES suit_manifests(id) ON DELETE CASCADE
	);

	-- Partial unique index to prevent duplicate active holdings (requires SQLite >= 3.8.0)
	CREATE UNIQUE INDEX IF NOT EXISTS uniq_agent_manifest_active ON agent_holding_suit_manifests(agent_id, suit_manifest_id) WHERE deleted = 0;
	`

	// Execute schema using transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CloseDB closes the database connection.
func CloseDB(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
