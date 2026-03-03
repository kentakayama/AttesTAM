//go:build integration

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
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
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/config"
	"github.com/kentakayama/tam-over-http/internal/infra/rats"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/veraison/eat"
	cose "github.com/veraison/go-cose"
	"github.com/veraison/swid"
)

func TestTAMResolveTEEPMessage_VERAISON_EAT_OK(t *testing.T) {
	logger := log.Default()
	verifierClient, err := rats.NewVerifierClient(config.RAConfig{
		BaseURL:     "https://localhost:8443/",
		ContentType: `application/eat+cwt; eat_profile="urn:ietf:rfc:rfc9711"`,
		InsecureTLS: true,
		Timeout:     60 * time.Second,
		Logger:      logger,
	})
	require.Nil(t, err)
	tam, err := NewTAM(verifierClient, logger)
	if err != nil {
		t.Fatalf("NewTAM error: %v", err)
	}
	if err = tam.Init(); err != nil {
		t.Fatalf("TAM Init error: %v", err)
	}
	if err = tam.EnsureDefaultEntity(false); err != nil {
		t.Fatalf("TAM EnsureDefaultEntity error: %v", err)
	}
	if err = tam.EnsureDefaultTEEPAgent(false); err != nil {
		t.Fatalf("TAM EnsureDefaultTEEPAgent error: %v", err)
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

	// fixed Attester(Attesting Environment)'s key (should be registed to VERAISON)
	attesterKey := cose.Key{
		Type:      cose.KeyTypeEC2,
		Algorithm: cose.AlgorithmES256,
		Params: map[any]any{
			cose.KeyLabelEC2Curve: cose.CurveP256,
			cose.KeyLabelEC2X:     []byte{0x30, 0xa0, 0x42, 0x4c, 0xd2, 0x1c, 0x29, 0x44, 0x83, 0x8a, 0x2d, 0x75, 0xc9, 0x2b, 0x37, 0xe7, 0x6e, 0xa2, 0x0d, 0x9f, 0x00, 0x89, 0x3a, 0x3b, 0x4e, 0xee, 0x8a, 0x3c, 0x0a, 0xaf, 0xec, 0x3e},
			cose.KeyLabelEC2Y:     []byte{0xe0, 0x4b, 0x65, 0xe9, 0x24, 0x56, 0xd9, 0x88, 0x8b, 0x52, 0xb3, 0x79, 0xbd, 0xfb, 0xd5, 0x1e, 0xe8, 0x69, 0xef, 0x1f, 0x0f, 0xc6, 0x5b, 0x66, 0x59, 0x69, 0x5b, 0x6c, 0xce, 0x08, 0x17, 0x23},
			cose.KeyLabelEC2D:     []byte{0xf3, 0xbd, 0x0c, 0x07, 0xa8, 0x1f, 0xb9, 0x32, 0x78, 0x1e, 0xd5, 0x27, 0x52, 0xf6, 0x0c, 0xc8, 0x9a, 0x6b, 0xe5, 0xe5, 0x19, 0x34, 0xfe, 0x01, 0x93, 0x8d, 0xdb, 0x55, 0xd8, 0xf7, 0x78, 0x01},
		},
	}
	attesterSigner, err := attesterKey.Signer()
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
	ueidBytes := append([]byte{0x01}, []byte("building-dev-123")...)
	ueid := eat.UEID(ueidBytes)
	var scheme swid.VersionScheme
	scheme.SetCode(swid.VersionSchemeMultipartNumeric)
	componentID := eat.ComponentID{
		Name: "TEEP Agent",
		Version: &eat.Version{
			Version: "1.3.4",
			Scheme:  &scheme,
		},
	}
	digest := eat.Digest{
		Alg: 1, // SHA-256
		Val: []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF},
	}
	measuredComponent := eat.MeasuredComponent{
		Id:          componentID,
		Measurement: &digest,
	}
	encodedMeasuredComponent, err := cbor.Marshal(measuredComponent)
	require.Nil(t, err)
	measurement := []eat.Measurement{
		{
			Type:   600, // TBD
			Format: eat.B64Url(encodedMeasuredComponent),
		},
	}

	evidence := eat.Eat{
		Nonce:        &nonce,
		UEID:         &ueid,
		Measurements: &measurement,
	}
	evidence.Cnf = &eat.KeyConfirmation{
		Key: agentKey,
	}
	encodedEvidence, err := cbor.Marshal(evidence)
	logger.Printf("encoded Evidence: %s\n", hex.EncodeToString(encodedEvidence))
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
