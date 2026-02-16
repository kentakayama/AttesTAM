# TEEP Agent Status Handling in TAM

This document describes how TAM's TEEP Agent Status are updated and obtained from the TAM.

```mermaid
---
title: API Endpoints relating to TEEP Agent Status
---

flowchart TB

TAM -- 1. Agent Status<br/>2. SUIT Manifest Overviews --> TAMAdmin([TAM Admin])
TAM -- 1. Agent Status --> DeviceManager([Device Admin])
TEEPAgent([TEEP Agent]) -- 3. TEEP Message (Update) --> TAM
TAM[TAM]
```

For internal implementation details, see [TAM Status TEEP Agent Status (Internal Design)](./TAM_STATUS_TEEP_AGENT_STATUS.md).

## Why Is This Required?

This TAM manages the following status for each TEEP Agent:
- **the public key of a TEEP Agent**, used in the TEEP Protocol security wrapper (COSE_Sign1, ESP256)
- **which Trusted Components a TEEP Agent has**
- what kind of errors occurred in a TEEP Agent, and how they were resolved (or not)

## 1. Specification of AgentService Web API

URL | Method | Authorized Requester | Input | Output
--|--|--|--|--
`/AgentService/ListAgents` | `GET` | TAM Admin/<br/>Device Admin | no query | 200 OK: `[ + kid ]`<br/>204 No Content<br/>400 Bad Request
`/AgentService/GetAgentStatus` | `POST` | TAM Admin/<br/>Device Admin | `[ + kid]` | 200 OK: `[ + agent-status-record ]`<br/> 204 No Content<br/>400 Bad Request

### A) ListAgents Web API

This endpoint



### B) GetAgentStatus Web API
### Output Format

```cddl
;# import rfc9711 as eat

get-agent-status-output = [
  * agent-status-record,
]

agent-status-record = [
  kid: bstr .size 32,
  status: agent-status,
]

agent-status = {
  attributes-label => agent-attributes,
  installed-tc-label => [ * suit-manifest-overview ],
}

attributes-label = 1
installed-tc-label = 2

agent-attributes = {
  eat.ueid-label => eat.ueid-type,
}
```

You can find the CDDL definitions of dependings:
- [RFC 9711](https://datatracker.ietf.org/doc/html/rfc9711#name-payload-cddl) for `eat.ueid-*`
- [SUIT Manifest Store](./SUIT_MANIFEST_STORE.md#specification-of-suitmanifestserviceregistermanifest-web-api)

Example output:
```cbor-diag
[
  [
    'dummy-teep-agent-kid-for-dev-123',
    {
      / attributes / 1: {
        / ueid / 256: h'016275696C64696E672D6465762D313233' / 0x01 + 'building-dev-123' /
      },
      / installed-tc / 2: [
        [
          / SUIT_Component_Identifier: / << ['app1.wasm'] >>,
          / manifest-sequence-number: / 3
        ],
        [
          / SUIT_Component_Identifier: / << ['app2.wasm'] >>,
          / manifest-sequence-number: / 2
        ]
      ]
    }
  ]
]
```

## Public Key of TEEP Agent

<!--
鍵がどうやって登録されるのか、
-->

This TAM authenticates the TEEP Agent public key using remote attestation.
For now, [VERAISON](https://github.com/veraison) is used as a Verifier with Background-Check Model.
Other Verifiers or Passport Model may be used.

```mermaid
sequenceDiagram
    Participant Agent AS TEEP Agent
    Participant TAM
    Participant Verifier AS VERAISON

    note over Agent: generate (pubAgent, privAgent)
    Agent ->> TAM: session creation
    TAM ->> Agent: challenge
    note over Agent: generate Evidence(challenge, pubAgent)
    Agent ->> TAM: Evidence
    TAM ->> Verifier: Evidence
    note over Verifier: appraise the Evidence
    Verifier ->> TAM: Attestation Result(challenge, pubAgent)
    note over TAM: check challenge<br/>store pubAgent
    TAM ->> Agent: close session
```

This TAM requires the TEEP Agent to prove
- are you running in the TEE with genuine hardware?
- is your Evidence fresh, i.e. generated after my challenge?
- which key do you use in the TEEP Protocol messages?

After successful remote attestation, TAM receives the challenge and the TEEP Agent public key from the verifier.

## Trusted Components Held by the TEEP Agent

This TAM records Trusted Components (and their SUIT manifests) stored in TEEP Agents.
These records are useful for Device Owners (or Device Manager Admins) who want to keep Trusted Components up to date.
getAgentsでは・・・・Ｔｒｕｓｔｅｄ　Ｃｏｍｐｏｎｅｎｔの一覧。


## Limitations

<!--
tc-listは一番信頼する、SUIT Reportで明示的なもの、

complete: リアルタイムで一致している　unmatch
-->

However, this is **NOT always complete** for several reasons.
- some TEEP Agents may not report SUIT manifest processing results
- even when an agent sends SUIT reports, intermediaries between the TEEP Agent and TAM (such as an untrusted TEEP Broker) may drop the message
- a TEEP Agent may lose the Trusted Component and/or SUIT manifest because not all TEEs provide durable storage
- a TEEP Agent may remove a Trusted Component via `UnrequestTA` without notifying TAM

As a result, the TEEP Agent status in TAM means "expected Trusted Components held by TEEP Agents" or "the Trusted Components a TEEP Agent should have."
It is constructed from the following information:
- TAM's Messages
  - which Trusted Components had the TAM sent to the TEEP Agent
- Agent's Messages
  - SUIT reports recording how SUIT manifests were processed in `suit-reports` of TEEP Success or Error messages
    - with successful SUIT Report, the Trusted Components in the corresponding SUIT manifest should be held by the TEEP Agent
    - additionally, those in SUIT manifests with lower `suit-manifest-sequence-number` are removed
    - with failure SUIT Report, existing Trusted Components should be kept
  - `tc-list` of TEEP QueryResponse message contains current Trusted Components

```mermaid
flowchart LR
    Agent[TEEP Agent] -- tc-list --> TAM
    TAM -- SUIT manifests --> Agent
    Agent -- SUIT reports --> TAM
```

As a result, TAM may store TEEP Agent status like the following table:

TEEP Agent | SUIT Manifest | Trusted Component | Status
--|--|--|--
Agent-1 | Manifest-A-seq1 | Component-a0 | Installation reported with a SUIT Report
Agent-1 | Manifest-B-seq0 | Component-b0 | Sent but not reported
Agent-1 | Manifest-A-seq0 | Component-a0 | Updated with Manifest-A-seq1
Agent-1 | Manifest-A-seq0 | Component-a1 | Removed on successful update with Manifest-A-seq1
Agent-2 | Manifest-A-seq1 | Component-a0 | Reported with tc-list in QueryResponse
Agent-2 | Manifest-B-seq0 | Component-b0 | Removal reported with a SUIT Report
Agent-2 | Manifest-C-seq0 | Component-c0 | Installed but not in `tc-list`

> [!WARNING]
> As you can see the table above, the status of TEEP Agents could be complicated.
> For now, the TAM only accepts SUIT Manifest with exactly ONE Trusted Component and reports the Trusted Components with explicit successful SUIT Report to avoid implementation complexity.
> That's why [example-agent-status.diag](./examples/example-agent-status.diag) does not contain the details.
