/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/infra/rats"
	"github.com/kentakayama/tam-over-http/internal/infra/sqlite"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/kentakayama/tam-over-http/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/veraison/eat"
	"github.com/veraison/go-cose"
)

var (
	tcID = suit.ComponentID{
		{
			0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, // hello.txt
		},
	}
)

type MockEATVerifier struct{}

func (e *MockEATVerifier) Process(data []byte) (*rats.ProcessedAttestation, error) {
	ev := rats.Evidence{
		Type:  `application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"`,
		Value: "0oRDoQEmoFkB1qkZAQl4GGh0dHA6Ly9hcm0uY29tL3BzYS8yLjAuMBkJWgEZCVsZMAAZCVxYIGFjbWUtaW1wbGVtZW50YXRpb24taWQtMDAwMDAwMDAxGQldWCDerb7v3q2+796tvu/erb7v3q2+796tvu/erb7v3q2+7xkJX4OkAWJCTAJYIIdCj8UigD0xBl57zjzwP+R1CWYx5eB7vXoP3mDEzyXHBGUyLjEuMAVYIKy7Ecfk2iFyBVI85M4aJFrhojmuPGv9nnhx9+XYuuhrpAFkUFJvVAJYIAJjgpmJtv2VT3K6ry/GS8Li8B1pLU3nKYbqgI9umYE/BGUxLjMuNQVYIKy7Ecfk2iFyBVI85M4aJFrhojmuPGv9nnhx9+XYuuhrpAFkQVJvVAJYIKOl5xXwzFdKc8P5vrtrwk8y/9W2ezhyRMLJCdp3mhR4BGUwLjEuNAVYIKy7Ecfk2iFyBVI85M4aJFrhojmuPGv9nnhx9+XYuuhrClggQUp8F0FBs9DpodKK8xUg8NQimf6sQAfe2J1ormzZLxkZAQBYIQHO6657iSejIn5TA89eDx97NLtUKtclCsA/vN427C8VCBkJYHgYaHR0cHM6Ly9wc2EtdmVyaWZpZXIub3JnWECP4/gt1Bld896yqFSdxrDnaj+6t8K0x6lr0o7nJW1LvG60MJ5p/cRdb2TsIoVVrBRAMdwbQevYLrPPLCiurF4M",
	}
	resp := rats.AttestationResponse{
		Status:   "complete",
		Nonce:    "QUp8F0FBs9DpodKK8xUg8NQimf6sQAfe2J1ormzZLxk=",
		Expiry:   "2038-01-19T03:14:07.999999999Z",
		Accept:   []string{`application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"`},
		Evidence: ev,
		Result:   "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJlYXIudmVyaWZpZXItaWQiOnsiYnVpbGQiOiJOL0EiLCJkZXZlbG9wZXIiOiJWZXJhaXNvbiBQcm9qZWN0In0sImVhdF9ub25jZSI6IlFVcDhGMEZCczlEcG9kS0s4eFVnOE5RaW1mNnNRQWZlMkoxb3JtelpMeGs9IiwiZWF0X3Byb2ZpbGUiOiJ0YWc6Z2l0aHViLmNvbSwyMDIzOnZlcmFpc29uL2VhciIsImlhdCI6MTc2NTMyOTMwOCwic3VibW9kcyI6eyJQU0FfSU9UIjp7ImVhci5hcHByYWlzYWwtcG9saWN5LWlkIjoicG9saWN5OlBTQV9JT1QiLCJlYXIuc3RhdHVzIjoiYWZmaXJtaW5nIiwiZWFyLnRydXN0d29ydGhpbmVzcy12ZWN0b3IiOnsiY29uZmlndXJhdGlvbiI6MCwiZXhlY3V0YWJsZXMiOjIsImZpbGUtc3lzdGVtIjowLCJoYXJkd2FyZSI6MiwiaW5zdGFuY2UtaWRlbnRpdHkiOjIsInJ1bnRpbWUtb3BhcXVlIjoyLCJzb3VyY2VkLWRhdGEiOjAsInN0b3JhZ2Utb3BhcXVlIjoyfSwiZWFyLnZlcmFpc29uLmFubm90YXRlZC1ldmlkZW5jZSI6eyJlYXQtcHJvZmlsZSI6Imh0dHA6Ly9hcm0uY29tL3BzYS8yLjAuMCIsInBzYS1ib290LXNlZWQiOiIzcTIrNzk2dHZ1L2VyYjd2M3EyKzc5NnR2dS9lcmI3djNxMis3OTZ0dnU4PSIsInBzYS1jbGllbnQtaWQiOjEsInBzYS1pbXBsZW1lbnRhdGlvbi1pZCI6IllXTnRaUzFwYlhCc1pXMWxiblJoZEdsdmJpMXBaQzB3TURBd01EQXdNREU9IiwicHNhLWluc3RhbmNlLWlkIjoiQWM3cnJudUpKNk1pZmxNRHoxNFBIM3MwdTFRcTF5VUt3RCs4M2pic0x4VUkiLCJwc2Etbm9uY2UiOiJRVXA4RjBGQnM5RHBvZEtLOHhVZzhOUWltZjZzUUFmZTJKMW9ybXpaTHhrPSIsInBzYS1zZWN1cml0eS1saWZlY3ljbGUiOjEyMjg4LCJwc2Etc29mdHdhcmUtY29tcG9uZW50cyI6W3sibWVhc3VyZW1lbnQtdHlwZSI6IkJMIiwibWVhc3VyZW1lbnQtdmFsdWUiOiJoMEtQeFNLQVBURUdYbnZPUFBBLzVIVUpaakhsNEh1OWVnL2VZTVRQSmNjPSIsInNpZ25lci1pZCI6InJMc1J4K1RhSVhJRlVqemt6aG9rV3VHaU9hNDhhLzJlZUhIMzVkaTY2R3M9IiwidmVyc2lvbiI6IjIuMS4wIn0seyJtZWFzdXJlbWVudC10eXBlIjoiUFJvVCIsIm1lYXN1cmVtZW50LXZhbHVlIjoiQW1PQ21ZbTIvWlZQY3Jxdkw4Wkx3dUx3SFdrdFRlY3BodXFBajI2WmdUOD0iLCJzaWduZXItaWQiOiJyTHNSeCtUYUlYSUZVanpremhva1d1R2lPYTQ4YS8yZWVISDM1ZGk2NkdzPSIsInZlcnNpb24iOiIxLjMuNSJ9LHsibWVhc3VyZW1lbnQtdHlwZSI6IkFSb1QiLCJtZWFzdXJlbWVudC12YWx1ZSI6Im82WG5GZkRNVjBwencvbSt1MnZDVHpMLzFiWjdPSEpFd3NrSjJuZWFGSGc9Iiwic2lnbmVyLWlkIjoickxzUngrVGFJWElGVWp6a3pob2tXdUdpT2E0OGEvMmVlSEgzNWRpNjZHcz0iLCJ2ZXJzaW9uIjoiMC4xLjQifV0sInBzYS12ZXJpZmljYXRpb24tc2VydmljZS1pbmRpY2F0b3IiOiJodHRwczovL3BzYS12ZXJpZmllci5vcmcifX19fQ.Vw4g5Th1oUXLnHsxLopSRWJhowCeqSc6SVgkJZSHGToZgK3Xe7nciMYJVlQq7o8wryR_MCg0_dz3BMq9rieUBQ",
	}
	ret := rats.ProcessedAttestation{
		Response:   &resp,
		EarStatus:  "affirming",
		SendUpdate: true,
	}
	return &ret, nil
}

func TestTAMResolveTEEPMessage_AgentAttestation_OK(t *testing.T) {
	logger := log.Default()
	verifier := MockEATVerifier{}
	tam, err := NewTAM(&verifier, logger)
	if err != nil {
		t.Fatalf("NewTAM error: %v", err)
	}
	if err = tam.InitWithPath(":memory:"); err != nil {
		t.Fatalf("TAM Init error: %v", err)
	}
	if err = tam.EnsureDefaultEntity(false); err != nil {
		t.Fatalf("TAM EnsureDefaultEntity error: %v", err)
	}
	// tam.EnsureDefaultTEEPAgent is not required, because EAT can carry the public key of the TEEP Agent

	kid, err := tam.tamKey.Thumbprint(crypto.SHA256)
	fmt.Printf("TAM's kid: %s\n", hex.EncodeToString(kid))
	require.Nil(t, err)
	require.NotNil(t, kid)

	// generate TEEP Agent's key
	ecdsaAgentKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)
	agentKey, err := cose.NewKeyEC2(cose.AlgorithmESP256, ecdsaAgentKey.PublicKey.X.Bytes(), ecdsaAgentKey.PublicKey.Y.Bytes(), ecdsaAgentKey.D.Bytes())
	require.Nil(t, err)
	agentKID, err := agentKey.Thumbprint(crypto.SHA256)
	require.Nil(t, err)

	// generate Attester(Attesting Environment)'s key
	ecdsaAttesterKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)
	attesterSigner, err := cose.NewSigner(cose.AlgorithmES256, ecdsaAttesterKey)
	require.Nil(t, err)

	// TEST#1: process empty body to return QueryRequest with Token
	responseEmpty, err := tam.ResolveTEEPMessage(nil)
	require.Nil(t, err)

	var outgoingQueryRequestWithToken TEEPMessage
	err = outgoingQueryRequestWithToken.COSESign1Verify(tam.tamKey, responseEmpty)
	require.Nil(t, err)
	assert.Equal(t, TEEPTypeQueryRequest, outgoingQueryRequestWithToken.Type)

	// TEST#2: generate TEEP Agent's QueryResponse with Token
	assert.Nil(t, outgoingQueryRequestWithToken.Options.Challenge)
	require.NotNil(t, outgoingQueryRequestWithToken.Options.Token)
	assert.Equal(t, true, outgoingQueryRequestWithToken.DataItemRequested.TCListRequested())
	queryResponseWithTCList := TEEPMessage{
		Type: TEEPTypeQueryResponse,
		Options: TEEPOptions{
			Token: outgoingQueryRequestWithToken.Options.Token,
			TCList: []suit.SystemPropertyClaims{
				{
					SystemComponentID: tcID,
				},
			},
		},
	}
	signedQueryResponseWithTCList, err := queryResponseWithTCList.COSESign1Sign(agentKey)
	require.Nil(t, err)

	// TEST#3: process QueryRequest with Token to return QueryRequest with Challenge
	responseTCList, err := tam.ResolveTEEPMessage(signedQueryResponseWithTCList)
	require.Nil(t, err)
	require.NotNil(t, responseTCList)

	var outgoingQueryRequestWithChallenge TEEPMessage
	err = outgoingQueryRequestWithChallenge.COSESign1Verify(tam.tamKey, responseTCList)
	require.Nil(t, err)
	assert.Equal(t, TEEPTypeQueryRequest, outgoingQueryRequestWithChallenge.Type)

	// TEST#4: generate TEEP Agent's QueryResponse with Challenge
	assert.Nil(t, outgoingQueryRequestWithChallenge.Options.Token)
	require.NotNil(t, outgoingQueryRequestWithChallenge.Options.Challenge)
	nonce := eat.Nonce{}
	nonce.Add(outgoingQueryRequestWithChallenge.Options.Challenge)
	evidence := eat.Eat{
		Nonce: &nonce,
	}
	evidence.Cnf = &eat.KeyConfirmation{
		Key: agentKey,
	}
	encodedEvidence, err := cbor.Marshal(evidence)
	require.Nil(t, err)
	// create message header
	headers := cose.Headers{
		Protected: cose.ProtectedHeader{
			cose.HeaderLabelAlgorithm: cose.AlgorithmES256,
		},
	}
	signedEvidence, err := cose.Sign1(rand.Reader, attesterSigner, headers, encodedEvidence, nil)
	require.Nil(t, err)
	logger.Printf("signed Evidence: %s\n", hex.EncodeToString(signedEvidence))
	queryResponse := TEEPMessage{
		Type: TEEPTypeQueryResponse,
		Options: TEEPOptions{
			AttestationPayload: signedEvidence,
		},
	}
	signedQueryResponseWithEvidence, err := queryResponse.COSESign1Sign(agentKey)

	// TEST#5: process QueryResponse with Evidence to return empty
	responseEvidence, err := tam.ResolveTEEPMessage(signedQueryResponseWithEvidence)
	require.Nil(t, err)
	assert.Nil(t, responseEvidence)
	// make sure that the key is trusted while resolving QueryResponse with EAT Cnf claim

	// TEST#3b: confirm stored TEEP Agent's key
	key, err := tam.getTEEPAgentKey(agentKID)
	require.Nil(t, err)
	require.Equal(t, cose.AlgorithmESP256, key.Algorithm)
	ckt, err := key.Thumbprint(crypto.SHA256)
	require.Nil(t, err)
	require.Equal(t, agentKID, ckt)
}

func TestTAMResolveTEEPMessage_AgentUpdate_OK(t *testing.T) {
	testTAMResolveTEEPMessage_AgentUpdate_OK(t, true)
	testTAMResolveTEEPMessage_AgentUpdate_OK(t, false)
}

func testTAMResolveTEEPMessage_AgentUpdate_OK(t *testing.T, success bool) {
	logger := log.Default()
	verifier := MockEATVerifier{}
	tam, err := NewTAM(&verifier, logger)
	if err != nil {
		t.Fatalf("NewTAM error: %v", err)
	}
	if err = tam.InitWithPath(":memory:"); err != nil {
		t.Fatalf("TAM Init error: %v", err)
	}
	if err = tam.EnsureDefaultEntity(true); err != nil {
		t.Fatalf("TAM EnsureDefaultEntity error: %v", err)
	}
	if err = tam.EnsureDefaultTEEPAgent(true); err != nil {
		t.Fatalf("TAM EnsureDefaultTEEPAgent error: %v", err)
	}

	kid, err := tam.tamKey.Thumbprint(crypto.SHA256)
	fmt.Printf("TAM's kid: %s\n", hex.EncodeToString(kid))
	require.Nil(t, err)
	require.NotNil(t, kid)

	// get TEEP Agent's key
	agentKID := []byte{
		0xd0, 0x8d, 0x16, 0x02, 0xca, 0xa2, 0xd0, 0xae, 0x0a, 0xde, 0x02, 0x66, 0x62, 0x92, 0xb1, 0x4c,
		0xef, 0xd0, 0xd0, 0x28, 0x2a, 0x15, 0x3f, 0x77, 0x73, 0xac, 0xf6, 0xfd, 0xd0, 0xc0, 0xd3, 0x78,
	}
	agentKey, err := tam.getTEEPAgentKey(agentKID)
	require.Nil(t, err)

	// TEST#1: process empty body to return QueryRequest with Token
	responseEmpty, err := tam.ResolveTEEPMessage(nil)
	require.Nil(t, err)

	var outgoingQueryRequestWithToken TEEPMessage
	err = outgoingQueryRequestWithToken.COSESign1Verify(tam.tamKey, responseEmpty)
	require.Nil(t, err)
	assert.Equal(t, TEEPTypeQueryRequest, outgoingQueryRequestWithToken.Type)

	// TEST#2: generate TEEP Agent's QueryResponse with Token
	assert.Nil(t, outgoingQueryRequestWithToken.Options.Challenge)
	require.NotNil(t, outgoingQueryRequestWithToken.Options.Token)
	assert.Equal(t, true, outgoingQueryRequestWithToken.DataItemRequested.TCListRequested())
	queryResponseWithTCList := TEEPMessage{
		Type: TEEPTypeQueryResponse,
		Options: TEEPOptions{
			Token: outgoingQueryRequestWithToken.Options.Token,
			TCList: []suit.SystemPropertyClaims{
				{
					SystemComponentID: tcID,
				},
			},
			RequestedTCList: []RequestedTCInfo{
				{
					ComponentID: tcID,
					// latest
				},
			},
		},
	}
	signedQueryResponseWithTCList, err := queryResponseWithTCList.COSESign1Sign(agentKey)
	require.Nil(t, err)

	// TEST#3: process QueryRequest with Token to return Update
	responseTCList, err := tam.ResolveTEEPMessage(signedQueryResponseWithTCList)
	require.Nil(t, err)
	require.NotNil(t, responseTCList)

	var outgoingUpdate TEEPMessage
	err = outgoingUpdate.COSESign1Verify(tam.tamKey, responseTCList)
	require.Nil(t, err)
	assert.Equal(t, TEEPTypeUpdate, outgoingUpdate.Type)

	if success {
		// TEST#4a: generate TEEP Agent's Success with Token
		require.NotNil(t, outgoingUpdate.Options.Token)
		require.Equal(t, 1, len(outgoingUpdate.Options.ManifestList))
		var envelope suit.Envelope
		err = cbor.Unmarshal(outgoingUpdate.Options.ManifestList[0], &envelope)
		require.Nil(t, err)
		var digest suit.Digest
		err = cbor.Unmarshal(envelope.AuthenticationWrapper.Value.DigestBstr, &digest)
		require.Nil(t, err)
		result := true
		report := suit.Report{
			Reference: suit.Reference{
				ManifestUri: "",
				Digest:      digest,
			},
			Result: suit.TrueOrDetailedResult{
				True: &result,
			},
		}
		encodedReport, err := cbor.Marshal(report)
		require.Nil(t, err)
		successWithReport := TEEPMessage{
			Type: TEEPTypeSuccess,
			Options: TEEPOptions{
				Token:       outgoingUpdate.Options.Token,
				SUITReports: util.DiagList[util.BytesHexMax32]{util.BytesHexMax32(encodedReport)},
			},
		}
		signedSuccess, err := successWithReport.COSESign1Sign(agentKey)
		require.Nil(t, err)

		// TEST#5a: process Success with Token and SUIT_Report to return empty
		responseSuccess, err := tam.ResolveTEEPMessage(signedSuccess)
		require.Nil(t, err)
		assert.Nil(t, responseSuccess)

		// TEST#6a: check agent have ONE successful SUIT Manifest
		entityRepo := sqlite.NewEntityRepository(tam.db)
		adm, err := entityRepo.FindByName(tam.ctx, "admin@example.com")
		require.Nil(t, err)
		agentStatus, err := tam.GetAgentStatus(adm, agentKID)
		require.Nil(t, err)
		require.NotNil(t, agentStatus)
		require.Equal(t, 1, len(agentStatus.Status.SuitManifests))
		encodedComponentID, err := cbor.Marshal(tcID)
		require.Nil(t, err)
		require.Equal(t, encodedComponentID, agentStatus.Status.SuitManifests[0].TrustedComponentID)
		require.Equal(t, uint64(1), agentStatus.Status.SuitManifests[0].SequenceNumber)
	} else {
		// TEST#4a: generate TEEP Agent's Error with Token
		require.NotNil(t, outgoingUpdate.Options.Token)
		require.Equal(t, 1, len(outgoingUpdate.Options.ManifestList))
		var envelope suit.Envelope
		err = cbor.Unmarshal(outgoingUpdate.Options.ManifestList[0], &envelope)
		require.Nil(t, err)
		var digest suit.Digest
		err = cbor.Unmarshal(envelope.AuthenticationWrapper.Value.DigestBstr, &digest)
		require.Nil(t, err)
		result := suit.DetailedResult{
			ResultReason: suit.ReportReasonUnauthorised,
		}
		report := suit.Report{
			Reference: suit.Reference{
				ManifestUri: "",
				Digest:      digest,
			},
			Result: suit.TrueOrDetailedResult{
				DetailedResult: &result,
			},
		}
		encodedReport, err := cbor.Marshal(report)
		require.Nil(t, err)
		errorWithReport := TEEPMessage{
			Type: TEEPTypeError,
			Options: TEEPOptions{
				Token:       outgoingUpdate.Options.Token,
				SUITReports: util.DiagList[util.BytesHexMax32]{util.BytesHexMax32(encodedReport)},
			},
		}
		signedError, err := errorWithReport.COSESign1Sign(agentKey)
		require.Nil(t, err)

		// TEST#5b: process Error with Token and SUIT_Report to return empty
		responseError, err := tam.ResolveTEEPMessage(signedError)
		require.Nil(t, err)
		assert.Nil(t, responseError)

		// TEST#6b: check agent have NO successful SUIT Manifest
		entityRepo := sqlite.NewEntityRepository(tam.db)
		adm, err := entityRepo.FindByName(tam.ctx, "admin@example.com")
		require.Nil(t, err)
		agentStatus, err := tam.GetAgentStatus(adm, agentKID)
		require.Nil(t, err)
		require.NotNil(t, agentStatus)
		require.Equal(t, 0, len(agentStatus.Status.SuitManifests))
	}
}

func TestTAMResolveTEEPMessage_TokenConsumed(t *testing.T) {
	logger := log.Default()
	verifier := MockEATVerifier{}
	tam, err := NewTAM(&verifier, logger)
	if err != nil {
		t.Fatalf("NewTAM error: %v", err)
	}
	if err = tam.InitWithPath(":memory:"); err != nil {
		t.Fatalf("TAM Init error: %v", err)
	}
	if err = tam.EnsureDefaultEntity(true); err != nil {
		t.Fatalf("TAM EnsureDefaultEntity error: %v", err)
	}
	if err = tam.EnsureDefaultTEEPAgent(false); err != nil {
		t.Fatalf("TAM EnsureDefaultTEEPAgent error: %v", err)
	}

	kid, err := tam.tamKey.Thumbprint(crypto.SHA256)
	fmt.Printf("TAM's kid: %s\n", hex.EncodeToString(kid))
	require.Nil(t, err)
	require.NotNil(t, kid)

	// get TEEP Agent's key
	agentKID := []byte{
		0xd0, 0x8d, 0x16, 0x02, 0xca, 0xa2, 0xd0, 0xae, 0x0a, 0xde, 0x02, 0x66, 0x62, 0x92, 0xb1, 0x4c,
		0xef, 0xd0, 0xd0, 0x28, 0x2a, 0x15, 0x3f, 0x77, 0x73, 0xac, 0xf6, 0xfd, 0xd0, 0xc0, 0xd3, 0x78,
	}
	agentKey, err := tam.getTEEPAgentKey(agentKID)
	require.Nil(t, err)

	// TEST#1: process empty body to return QueryRequest with Token
	responseEmpty, err := tam.ResolveTEEPMessage(nil)
	require.Nil(t, err)

	var outgoingQueryRequestWithToken TEEPMessage
	err = outgoingQueryRequestWithToken.COSESign1Verify(tam.tamKey, responseEmpty)
	require.Nil(t, err)
	assert.Equal(t, TEEPTypeQueryRequest, outgoingQueryRequestWithToken.Type)

	// TEST#2: generate TEEP Agent's QueryResponse with Token
	assert.Nil(t, outgoingQueryRequestWithToken.Options.Challenge)
	require.NotNil(t, outgoingQueryRequestWithToken.Options.Token)
	assert.Equal(t, true, outgoingQueryRequestWithToken.DataItemRequested.TCListRequested())
	queryResponse := TEEPMessage{
		Type: TEEPTypeQueryResponse,
		Options: TEEPOptions{
			Token: outgoingQueryRequestWithToken.Options.Token,
		},
	}
	signedQueryResponse, err := queryResponse.COSESign1Sign(agentKey)
	require.Nil(t, err)

	// TEST#3: process QueryRequest with Token to return Update
	firstEmpty, err := tam.ResolveTEEPMessage(signedQueryResponse)
	require.Nil(t, err)
	assert.Nil(t, firstEmpty)

	// TEST#4: process the same QueryRequest to return error
	// since the handler tries to process the attestation-payload (Evidence / AttestationResults),
	// the error will be ErrAttestationPayloadNotFound
	secondEmpty, err := tam.ResolveTEEPMessage(signedQueryResponse)
	require.Equal(t, ErrAttestationPayloadNotFound, err)
	assert.Nil(t, secondEmpty)
}

func TestTAMEnsureDefaultTEEPAgent_Dummy_OK(t *testing.T) {
	logger := log.Default()
	verifier := MockEATVerifier{}
	tam, err := NewTAM(&verifier, logger)
	if err != nil {
		t.Fatalf("NewTAM error: %v", err)
	}
	if err = tam.InitWithPath(":memory:"); err != nil {
		t.Fatalf("TAM Init error: %v", err)
	}
	if err = tam.EnsureDefaultEntity(true); err != nil {
		t.Fatalf("TAM EnsureDefaultEntity error: %v", err)
	}
	if err = tam.EnsureDefaultTEEPAgent(true); err != nil {
		t.Fatalf("TAM EnsureDefaultTEEPAgent error: %v", err)
	}

	agentKID := []byte("dummy-teep-agent-kid-for-dev-123")
	ueid := append([]byte{0x01}, []byte("building-dev-123")...)
	agentStatusRepo := sqlite.NewAgentStatusRepository(tam.db)
	agentStatus, err := agentStatusRepo.GetAgentStatus(tam.ctx, agentKID)
	require.Nil(t, err)
	require.NotNil(t, agentStatus)
	assert.Equal(t, agentKID, agentStatus.AgentKID)
	assert.Equal(t, ueid, agentStatus.DeviceUEID)
	assert.Equal(t, 2, len(agentStatus.SuitManifests))
	var record AgentStatusRecord
	err = record.FromModel(agentStatus)
	require.Nil(t, err)

	expected := []byte{
		0x81, 0x82, 0x58, 0x20, 0x64, 0x75, 0x6D, 0x6D, 0x79, 0x2D, 0x74, 0x65, 0x65, 0x70, 0x2D, 0x61,
		0x67, 0x65, 0x6E, 0x74, 0x2D, 0x6B, 0x69, 0x64, 0x2D, 0x66, 0x6F, 0x72, 0x2D, 0x64, 0x65, 0x76,
		0x2D, 0x31, 0x32, 0x33, 0xA2, 0x69, 0x77, 0x61, 0x70, 0x70, 0x5F, 0x6C, 0x69, 0x73, 0x74, 0x82,
		0x82, 0x4B, 0x81, 0x49, 0x61, 0x70, 0x70, 0x31, 0x2E, 0x77, 0x61, 0x73, 0x6D, 0x03, 0x82, 0x4B,
		0x81, 0x49, 0x61, 0x70, 0x70, 0x32, 0x2E, 0x77, 0x61, 0x73, 0x6D, 0x02, 0x6A, 0x61, 0x74, 0x74,
		0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0xA1, 0x19, 0x01, 0x00, 0x51, 0x01, 0x62, 0x75, 0x69,
		0x6C, 0x64, 0x69, 0x6E, 0x67, 0x2D, 0x64, 0x65, 0x76, 0x2D, 0x31, 0x32, 0x33,
	}

	encoded, err := cbor.Marshal([]AgentStatusRecord{record})
	require.Nil(t, err)
	assert.Equal(t, expected, encoded)
}
