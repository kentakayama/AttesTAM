/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/infra/sqlite"
)

type AgentStatusRecord struct {
	AgentKID []byte
	Status   AgentStatus
}

type AgentStatus struct {
	Attributes    AgentAttributes              `cbor:"attributes"`
	SuitManifests []model.SuitManifestOverview `cbor:"wapp_list"`
	// UpdatedAt     time.Time                    `cbor:"updated_at,omitempty"`
}

type AgentAttributes struct {
	DeviceUEID []byte `cbor:"256,keyasint,omitempty"`
}

func (s *AgentStatusRecord) MarshalCBOR() ([]byte, error) {
	opts := cbor.EncOptions{
		Sort: cbor.SortCoreDeterministic,
	}
	em, err := opts.EncMode()
	if err != nil {
		return nil, err
	}

	return em.Marshal([]any{s.AgentKID, s.Status})
}

func (r *AgentStatusRecord) FromModel(agentStatus *model.AgentStatus) error {
	if agentStatus == nil {
		return fmt.Errorf("Agent Status is nil")
	}
	r.AgentKID = agentStatus.AgentKID
	if agentStatus.DeviceUEID != nil {
		r.Status.Attributes.DeviceUEID = agentStatus.DeviceUEID
	}
	if len(agentStatus.SuitManifests) != 0 {
		r.Status.SuitManifests = agentStatus.SuitManifests
	}

	return nil
}

func (t *TAM) GetAgentStatus(entity *model.Entity, agentKID []byte) (*AgentStatusRecord, error) {
	// TODO: switch query based on the entity role

	arepo := sqlite.NewAgentStatusRepository(t.db)

	agentStatus, err := arepo.GetAgentStatus(t.ctx, agentKID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent status: %w", err)
	}
	if agentStatus == nil {
		// not found
		return nil, nil
	}

	var record AgentStatusRecord
	if err := record.FromModel(agentStatus); err != nil {
		return nil, fmt.Errorf("failed to convert agent status: %w", err)
	}

	return &record, nil
}

func (t *TAM) GetAgentStatuses(entity *model.Entity) ([]*AgentStatusRecord, error) {
	// TODO: switch query based on the entity role

	arepo := sqlite.NewAgentRepository(t.db)
	agents, err := arepo.GetAll(t.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agentStatuses := make([]*AgentStatusRecord, 0, len(agents))
	astatusRepo := sqlite.NewAgentStatusRepository(t.db)
	for _, agent := range agents {
		agentStatus, err := astatusRepo.GetAgentStatus(t.ctx, agent.KID)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent status for agent KID %x: %w", agent.KID, err)
		}
		var record AgentStatusRecord
		if err := record.FromModel(agentStatus); err != nil {
			return nil, fmt.Errorf("failed to convert agent status: %w", err)
		}
		agentStatuses = append(agentStatuses, &record)
	}

	return agentStatuses, nil
}

func (t *TAM) updateAgentStatusOnManifestSuccess(agentKID []byte, manifestDigest []byte, reportBytes []byte) {
	arepo := sqlite.NewAgentStatusRepository(t.db)

	arepo.ReflectManifestSuccess(t.ctx, agentKID, manifestDigest, reportBytes)
}

func (t *TAM) updateAgentStatusOnManifestError(agentKID []byte, manifestDigest []byte, reportBytes []byte) {
	arepo := sqlite.NewAgentStatusRepository(t.db)

	arepo.RecordManifestProcessingFailure(t.ctx, agentKID, manifestDigest, reportBytes)
}
