/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"fmt"

	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/infra/sqlite"
)

func (t *TAM) GetAgentStatus(entity *model.Entity, agentKID []byte) (*model.AgentStatus, error) {
	// TODO: switch query based on the entity role

	arepo := sqlite.NewAgentStatusRepository(t.db)

	agentStatus, err := arepo.GetAgentStatus(t.ctx, agentKID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent statuses: %w", err)
	}

	return agentStatus, nil
}

func (t *TAM) GetAgentStatuses(entity *model.Entity) ([]*model.AgentStatus, error) {
	// TODO: switch query based on the entity role

	arepo := sqlite.NewAgentRepository(t.db)
	agents, err := arepo.GetAll(t.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agentStatuses := make([]*model.AgentStatus, 0, len(agents))
	astatusRepo := sqlite.NewAgentStatusRepository(t.db)
	for _, agent := range agents {
		agentStatus, err := astatusRepo.GetAgentStatus(t.ctx, agent.KID)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent status for agent KID %x: %w", agent.KID, err)
		}
		agentStatuses = append(agentStatuses, agentStatus)
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
