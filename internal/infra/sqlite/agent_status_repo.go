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
	"time"

	"github.com/kentakayama/tam-over-http/internal/domain/model"
)

// AgentStatusRepository handles agent -> manifest holdings.
type AgentStatusRepository struct {
	db *sql.DB
}

func NewAgentStatusRepository(db *sql.DB) *AgentStatusRepository {
	return &AgentStatusRepository{db: db}
}

// AddForAgent logically deletes existing holdings for the same trusted_component and inserts a new holding.
// This operation is performed in a transaction to ensure atomicity.
// After successful insertion, it updates the agent's updated_at timestamp.
func (r *AgentStatusRepository) AddForAgent(ctx context.Context, agentKID []byte, SuitManifestDigest []byte) error {
	// Find agent ID and manifest ID and trusted_component_id in a single query
	var agentID int64
	var manifestID int64
	var trustedComponentID []byte
	const lookupQuery = `
		SELECT a.id, sm.id, sm.trusted_component_id
		FROM agents a, suit_manifests sm
		WHERE a.kid = ? AND sm.digest = ?
		LIMIT 1
	`
	err := r.db.QueryRowContext(ctx, lookupQuery, agentKID, SuitManifestDigest).Scan(&agentID, &manifestID, &trustedComponentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("agent or manifest not found")
		}
		return fmt.Errorf("lookup agent and manifest: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Mark existing active holdings for this agent and trusted_component as deleted
	upd := `
		UPDATE agent_holding_suit_manifests
		SET deleted = 1
		WHERE agent_id = ?
		  AND deleted = 0
		  AND suit_manifest_id IN (
			SELECT id FROM suit_manifests WHERE trusted_component_id = ?
		  )
	`
	if _, err := tx.ExecContext(ctx, upd, agentID, trustedComponentID); err != nil {
		return fmt.Errorf("mark deleted: %w", err)
	}

	// Insert the new holding
	ins := `
		INSERT INTO agent_holding_suit_manifests (agent_id, suit_manifest_id, created_at, deleted)
		VALUES (?, ?, CURRENT_TIMESTAMP, 0)
	`
	if _, err := tx.ExecContext(ctx, ins, agentID, manifestID); err != nil {
		return fmt.Errorf("insert holding: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// GetAgentStatus retrieves the current status of an agent with its active SUIT manifest holdings.
// For each trusted_component_id, only the latest (by created_at) holding is returned.
// If a device is associated with the agent, its UEID is included in the response.
func (r *AgentStatusRepository) GetAgentStatus(ctx context.Context, agentKID []byte) (*model.AgentStatus, error) {
	// TODO: check the requesters' authorization to view the agent status.
	const q = `
		SELECT a.id, a.kid, a.created_at,
		       sm.trusted_component_id, sm.sequence_number,
		       d.ueid
		FROM agents a
		LEFT JOIN devices d ON a.device_id = d.id
		LEFT JOIN (
			SELECT ahsm.agent_id, ahsm.suit_manifest_id, sm.trusted_component_id, sm.sequence_number, ahsm.created_at,
				   ROW_NUMBER() OVER (PARTITION BY sm.trusted_component_id ORDER BY ahsm.created_at DESC) as rn
			FROM agent_holding_suit_manifests ahsm
			LEFT JOIN suit_manifests sm ON ahsm.suit_manifest_id = sm.id
			WHERE ahsm.deleted = 0
		) latest ON a.id = latest.agent_id AND latest.rn = 1
		LEFT JOIN suit_manifests sm ON latest.suit_manifest_id = sm.id
		WHERE a.kid = ?
	`
	rows, err := r.db.QueryContext(ctx, q, agentKID)
	if err != nil {
		return nil, fmt.Errorf("query agent and holdings: %w", err)
	}
	defer rows.Close()

	var agentStatus *model.AgentStatus
	manifests := []model.SuitManifestOverview{}

	for rows.Next() {
		var agentID int64
		var kid []byte
		var createdAt time.Time
		var trustedComponentID sql.NullString
		var sequenceNumber sql.NullInt64
		var ueid []byte
		var ueidNull sql.NullString

		if err := rows.Scan(&agentID, &kid, &createdAt, &trustedComponentID, &sequenceNumber, &ueidNull); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if ueidNull.Valid {
			ueid = []byte(ueidNull.String)
		}

		if agentStatus == nil {
			agentStatus = &model.AgentStatus{
				AgentKID:      kid,
				DeviceUEID:    ueid,
				SuitManifests: []model.SuitManifestOverview{},
				UpdatedAt:     createdAt,
			}
		}

		if trustedComponentID.Valid && sequenceNumber.Valid {
			manifest := model.SuitManifestOverview{
				TrustedComponentID: []byte(trustedComponentID.String),
				SequenceNumber:     uint64(sequenceNumber.Int64),
			}
			manifests = append(manifests, manifest)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if agentStatus == nil {
		return nil, nil
	}

	agentStatus.SuitManifests = manifests
	return agentStatus, nil
}

// UpdateAgentUpdatedAt updates the updated_at timestamp for an agent by its KID.
func (r *AgentStatusRepository) UpdateAgentUpdatedAt(ctx context.Context, agentKID []byte) error {
	const q = `
		UPDATE agents
		SET updated_at = ?
		WHERE kid = ?
	`
	_, err := r.db.ExecContext(ctx, q, time.Now().UTC(), agentKID)
	if err != nil {
		return fmt.Errorf("update agent updated_at: %w", err)
	}
	return nil
}
