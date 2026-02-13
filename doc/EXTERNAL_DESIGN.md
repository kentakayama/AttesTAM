# TAM External Design

```mermaid
---
title: TAM over HTTP - External Conceptual Design
---

flowchart TB

TCDeveloper([TC Developer]) -- POST SUIT Manifest --> HTTPServer
TAMAdmin([TAM Admin]) -- GET AgentStatus<br/>GET SUIT Manifests --> HTTPServer
DeviceManager([Device Manager]) -- GET AgentStatus (planned) --> HTTPServer
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
GET | `/admin/getManifests` | TAM Admin | none | 200: `[overview of SUIT Manifest]` (CBOR) | [SUIT_MANIFEST_STORE](SUIT_MANIFEST_STORE.md)
POST | `/tc-developer/addManifest` | TC Developer | SUIT Manifest | 200: OK | [SUIT_MANIFEST_STORE](SUIT_MANIFEST_STORE.md)
GET | `/admin/getAgents` | TAM Admin | none | 200: Agent status (CBOR) | [TEEP_AGENT_STATUS](TEEP_AGENT_STATUS.md)
POST | `/tam` | TEEP Agent | empty<br/>QueryResponse<br/>Success<br/>Error | 200: QueryRequest<br/>200: Update / QueryRequest<br/>204: empty<br/>204: empty | [TEEP_MESSAGE_HANDLE](TEEP_MESSAGE_HANDLE.md)

> [!NOTE]
> Current admin endpoints return fixed demo-oriented records. Request-specific filtering and role-based authorization are still TODO.
