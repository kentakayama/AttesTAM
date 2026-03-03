/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/infra/sqlite"
	"github.com/kentakayama/tam-over-http/internal/util"
)

type AgentStatusKey struct {
	_         struct{}           `cbor:",toarray"`
	AgentKID  util.BytesHexMax32 `cbor:"0,keyasint"`
	UpdatedAt time.Time          `cbor:"1,keyasint"`
}

type AgentStatusRecord struct {
	_        struct{}           `cbor:",toarray"`
	AgentKID util.BytesHexMax32 `cbor:"0,keyasint"`
	Status   AgentStatus        `cbor:"1,keyasint"`
}

type AgentStatus struct {
	Attributes    AgentAttributes              `cbor:"1,keyasint"`
	SuitManifests []model.SuitManifestOverview `cbor:"2,keyasint"`
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

func (t *TAM) GetAgentStatuses(entity *model.Entity) ([]*AgentStatusKey, error) {
	// TODO: switch query based on the entity role, and get this info without joining with Agent table, to avoid unnecessary DB access for each agent.

	// XXX: returns dummy data for testing, and will be removed after implementing the actual DB access logic.
	agentStatus := &AgentStatusKey{
		AgentKID:  []byte("dummy-teep-agent-kid-for-dev-123"),
		UpdatedAt: time.Now(),
	}
	agentStatuses := []*AgentStatusKey{agentStatus}

	// arepo := sqlite.NewAgentRepository(t.db)
	// agents, err := arepo.GetAll(t.ctx)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to list agents: %w", err)
	// }

	// agentStatuses := make([]*AgentStatusKey, 0, len(agents))
	// astatusRepo := sqlite.NewAgentStatusRepository(t.db)
	// for _, agent := range agents {
	// 	agentStatus, err := astatusRepo.GetAgentStatus(t.ctx, agent.KID)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to get agent status for agent KID %x: %w", agent.KID, err)
	// 	}
	// 	if agentStatus == nil {
	// 		continue
	// 	}
	// 	agentStatuses = append(agentStatuses, &AgentStatusKey{
	// 		AgentKID:  agentStatus.AgentKID,
	// 		UpdatedAt: agentStatus.UpdatedAt,
	// 	})
	// }

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
