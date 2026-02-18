# TAM Status TEEP Agent Status (Internal Design)

## Purpose
This document describes the internal implementation of TEEP Agent status handling in TAM.
It focuses on persistence model, update paths (mainly TEEP Success), and read paths for `/*/AgentService/GetAgentStatus`.

Terminology note:
- **Agent Status** is used consistently for the API representation and its persisted source.

## Components
- `internal/server/handler.go`
  - Handles `GET /AgentService/ListAgents` and `POST /AgentService/GetAgentStatus`, and encodes status responses as CBOR
- `internal/tam/agent_status.go`
  - `GetAgentStatus(...)` / `GetAgentStatuses(...)`
  - `updateAgentStatusOnManifestSuccess(...)`
  - `updateAgentStatusOnManifestError(...)`
- `internal/tam/tam.go`
  - Calls status update routines while processing TEEP `Success`/`Error` messages with SUIT reports
- `internal/infra/sqlite/agent_status_repo.go`
  - SQL read/write implementation for holdings and report records

## Data Model
Core tables used by agent status logic:

- `agents`
  - agent identity (`kid`), optional device binding (`device_id`), key material
- `devices`
  - optional device identity (`ueid`) and admin ownership
- `suit_manifests`
  - source of trusted component and sequence metadata
- `agent_holding_suit_manifests`
  - current active manifest holdings per agent (`deleted=0` means active)
- `suit_reports`
  - processing result records from TEEP `Success` / `Error` flows

Key behavior:
1. Holdings are versioned logically by inserting a new active row and marking old rows deleted for same trusted component.
2. `GetAgentStatus` returns the latest active holding per trusted component.

## Write Flow (TEEP Success Path)
Main status update path is executed when TAM receives authenticated TEEP `Success` containing SUIT reports.

```mermaid
sequenceDiagram
    participant Agent as TEEP Agent
    participant T as tam.TAM
    participant R as AgentStatusRepository
    participant DB as sqlite

    Agent->>T: Success (SUIT reports)
    T->>T: parse each SUIT report and resolve manifest digest
    opt report indicates success
        T->>R: ReflectManifestSuccess(agentKID, manifestDigest, reportBytes)
        R->>DB: lookup agent + manifest + trusted_component
        R->>DB: mark previous active holdings deleted (same trusted_component)
        R->>DB: insert new active holding
        R->>DB: insert suit_reports(resolved=1)
    end
```

Notes:
- `ReflectManifestSuccess` is transactional (delete old active rows + insert new holding + insert report).
- Failure-report path exists via `RecordManifestProcessingFailure(...)` and inserts unresolved records into `suit_reports`.

## Read Flow (`/AgentService/*`)

### 1) `GET /AgentService/ListAgents`

```mermaid
sequenceDiagram
    participant Admin as TAM Admin
    participant H as server.handler
    participant T as tam.TAM

    Admin->>H: GET /AgentService/ListAgents (Accept: application/cbor)
    H->>T: Authorize entity (TODO)
    H->>T: GetAgentStatuses(entity)
    T-->>H: AgentStatusKey list
    H-->>Admin: 200 + [+[kid, last updated] ] (or 204 if not found)
```

> [!NOTE]
> Handler currently returns status for one fixed demo agent KID.

### 2) `POST /AgentService/GetAgentStatus`
```mermaid
sequenceDiagram
    participant Admin as TAM Admin
    participant H as server.handler
    participant T as tam.TAM
    participant R as AgentStatusRepository
    participant DB as sqlite

    Admin->>H: POST /AgentService/GetAgentStatus [+kid] (Accept: application/cbor)
    H->>H: Authorize entity and parse input KIDs (TODO)
    loop each kid in input
      H->>T: GetAgentStatus(entity, kid)
      T->>R: GetAgentStatus(kid)
      R->>DB: query agent + latest active holdings + device ueid
      DB-->>R: status rows
      R-->>T: AgentStatusRecord
      T-->>H: AgentStatusRecord
    end
    H-->>Admin: 200 + CBOR (or 204 if not found)
```

## Output Record Shape
Returned status is mapped to `AgentStatusRecord`:
- `[agent-kid, status]`
- `status.attributes.256` (UEID) when available
- `status.installed-tc`: list of `[trusted-component-id, sequence-number]`

See [TEEP_AGENT_STATUS.md](./TEEP_AGENT_STATUS.md) for CDDL and API-level output semantics.
