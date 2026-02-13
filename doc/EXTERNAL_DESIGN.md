# TAM External Design

```mermaid
---
title: TAM over HTTP - External Conceptual Design
---

flowchart TB

TCDeveloper([TC Developer]) -- GET SUIT Manifests<br/>POST SUIT Manifest --> HTTPServer
DeviceManager([Device Manager]) -- GET AgentStatus --> HTTPServer
TEEPAgent([TEEP Agent]) -- POST TEEP Message --> HTTPServer

subgraph TAM Server
    HTTPServer[HTTP Server]
    TAMManager[TAM Manager]
    TAMoverHTTP[TAM Message Handler]
    DBMS[DBMS]

    HTTPServer -- TEEP Message --> TAMoverHTTP
    HTTPServer -- Command/Query --> TAMManager
    TAMManager --> DBMS    
    TAMoverHTTP --> DBMS
    DBMS --> DB
end

```

Method | Endpoint | Requester | Input | Output | Reference
--|--|--|--|--|--
GET | `/tc-developer/getManifests` | TC Developer | `{TBD}` | 200: `[overview of SUIT Manifest]` | [SUIT_MANIFEST_STORE](SUIT_MANIFEST_STORE.md)
POST | `/tc-developer/addManifest` | TC Developer | SUIT Manifest | 200: OK | [SUIT_MANIFEST_STORE](SUIT_MANIFEST_STORE.md)
GET | `/admin/getAgents` | TAM Admin | `{TBD}` | 200: `{TBD}` (status of Agents under this TAM) | [TEEP_AGENT_STATUS](TEEP_AGENT_STATUS.md)
POST | `/tam` | TEEP Agent | empty<br/>QueryResponse<br/>Success<br/>Error | 200: QueryRequest<br/>200: Update / QueryRequest<br/>204: empty<br/>204: empty | [TEEP_MESSAGE_HANDLE](TEEP_MESSAGE_HANDLE.md)

> [!NOTE]
> Endpoints with `{TBD}` input currently return fixed demo data and do not yet fully reflect request-specific filters.
