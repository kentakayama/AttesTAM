/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"bytes"
	"context"
	"crypto"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/AttesTAM/internal/demo"
	"github.com/kentakayama/AttesTAM/internal/domain/model"
	"github.com/kentakayama/AttesTAM/internal/infra/rats"
	"github.com/kentakayama/AttesTAM/internal/infra/sqlite"
	"github.com/kentakayama/AttesTAM/internal/suit"
	"github.com/kentakayama/AttesTAM/internal/util"
	"github.com/veraison/go-cose"
)

type TAM struct {
	verifier rats.IRAVerifier
	tamKey   *cose.Key
	logger   *log.Logger
	db       *sql.DB         // Database connection for TAM state
	ctx      context.Context // Background context for database operations
}

func NewTAM(tamPrivateKeyPath string, verifier rats.IRAVerifier, logger *log.Logger) (*TAM, error) {
	if tamPrivateKeyPath == "" {
		return nil, errors.New("tam private key path is required")
	}
	keyBytes, err := os.ReadFile(tamPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TAM private key from path %s: %w", tamPrivateKeyPath, err)
	}

	return NewTAMFromKeyBytes(keyBytes, verifier, logger)
}

func NewTAMFromKeyBytes(keyBytes []byte, verifier rats.IRAVerifier, logger *log.Logger) (*TAM, error) {
	var key cose.Key
	err := cbor.Unmarshal(keyBytes, &key)
	if err != nil {
		return nil, errors.New("failed to load TAM's private key")
	}
	kid, _ := key.Thumbprint(crypto.SHA256)
	logger.Printf("TAM's Key = %s, kid = %s", util.PrintCOSEKey(&key), util.BytesHexMax32(kid))

	return &TAM{
		verifier: verifier,
		tamKey:   &key,
		logger:   logger,
	}, nil
}

func NewDemoTAM(verifier rats.IRAVerifier, logger *log.Logger) (*TAM, error) {
	if logger != nil {
		logger.Printf("[WARNING] Using public insecure demo TAM private key. This must not be used outside demo or tests.")
	}
	return NewTAMFromKeyBytes(demo.DemoTAMPrivateKeyCBOR, verifier, logger)
}

func (t *TAM) ResolveTEEPMessage(body []byte) ([]byte, error) {
	var response []byte
	if len(body) == 0 {
		// empty body means session creation, return QueryRequest
		t.logger.Printf("received empty message. return QueryRequest with token.")
		return t.generateQueryRequest()
	}

	incomingMessage, agentKID, err := t.tryAuthenticateTeepMessage(body)
	if (err != nil && err != ErrNotAuthenticated) || incomingMessage == nil {
		// failed to parse incoming message
		return nil, err
	}

	// search the sent message by myself with token
	sentMessage := t.searchSentMessageWithToken(incomingMessage.Options.Token)
	// NOTE: sentMessage may be nil because the incomingMessage does not contain the token
	// case 1) TAM sent QueryRequest with challenge & request-attestation
	// case 2) someone created malformed TEEP Protocol messages

	t.logger.Printf("received %s.", incomingMessage.CBORDiagString(0))

	switch incomingMessage.Type {
	case TEEPTypeQueryResponse:
		// NOTE: agentKID == nil (not authenticated) is acceptable, because the verification key might be provided by the Verifier
		if sentMessage != nil {
			if agentKID == nil {
				// Remote Attestation is required
				return t.generateQueryRequestWithAttestation()
			}
		} else {
			// attestation may be requested with challange i.e. the sent message does not contain token
			attestationResults, err := t.verifyAttestationPayload(incomingMessage)
			if err != nil {
				return nil, err
			}

			// if attestationResult status is affirming, extract key from attestiaonPayload
			if !strings.EqualFold(attestationResults.EarStatus, "affirming") {
				return nil, ErrAttestationFailed
			}

			if agentKID == nil {
				// Legacy Veraison path carries EAT in attestation-payload directly.
				if attestationResults.AttestationKey == nil {
					if err := rats.PopulateAttestedClaimsFromSign1(attestationResults, incomingMessage.Options.AttestationPayload); err != nil {
						t.logger.Printf("failed to extract attested claims: %v", err)
						return nil, ErrNotAuthenticated
					}
				}

				sentMessage = t.searchSentMessageWithChallenge(attestationResults.AttestationNonce)
				if sentMessage == nil {
					return nil, ErrNotAuthenticated
				}

				if attestationResults.AttestationKey == nil {
					t.logger.Printf("attestation public key missing in payload")
					return nil, ErrNotAuthenticated
				}
				key := attestationResults.AttestationKey

				// verify QueryResponse signature
				if err := verifyCOSESignature(body, key); err != nil {
					t.logger.Printf("query-response verification failed: %v", err)
					return nil, ErrNotAuthenticated
				}

				// store the public key for future use
				if err := t.setTEEPAgentKey(key, attestationResults.AttestationUEID); err != nil {
					t.logger.Printf("failed to store attestation public key: %v", err)
					return nil, ErrFatal
				} else {
					agentKID, _ = key.Thumbprint(crypto.SHA256)
					t.logger.Printf("stored attestation public key %s, kid %s in system keyring.", util.PrintCOSEKey(key), util.BytesHexMax32(agentKID))
				}
			}
		}

		// finally authenticated?
		if agentKID == nil || sentMessage == nil {
			return nil, ErrNotAuthenticated
		}

		response, err = t.processQueryResponse(incomingMessage, agentKID, sentMessage)
		if err != nil {
			return nil, err
		}

	case TEEPTypeSuccess:
		if agentKID == nil {
			return nil, ErrNotAuthenticated
		}
		if sentMessage == nil {
			return nil, ErrNotAResponse
		}

		// handle SUIT_Report
		for i := 0; i < len(incomingMessage.Options.SUITReports); i++ {
			var report suit.Report
			if err := cbor.Unmarshal(incomingMessage.Options.SUITReports[i], &report); err != nil {
				t.logger.Printf("failed to Unmarshal SUIT Report: %v", err)
				continue
			}
			if report.Reference.Digest.DigestAlg != cose.AlgorithmSHA256 ||
				len(report.Reference.Digest.DigestBytes) != crypto.SHA256.Size() {
				// malformed
				t.logger.Printf("digest of a SUIT Report is malformed: %v", report.Reference.Digest)
				continue
			}
			manifestDigest, err := cbor.Marshal(report.Reference.Digest)
			if err != nil {
				t.logger.Printf("failed to encode a SUIT_Digest in SUIT_Report: %v", err)
			}
			if (report.Result.True != nil && *report.Result.True == true) ||
				(report.Result.DetailedResult != nil && report.Result.DetailedResult.ResultReason == suit.ReportReasonOK) {
				// record as success
				t.updateAgentStatusOnManifestSuccess(agentKID, manifestDigest, incomingMessage.Options.SUITReports[i])
			}
		}

		response = nil

	case TEEPTypeError:
		if agentKID == nil {
			return nil, ErrNotAuthenticated
		}
		if sentMessage == nil {
			return nil, ErrNotAResponse
		}

		// handle SUIT_Report
		for i := 0; i < len(incomingMessage.Options.SUITReports); i++ {
			var report suit.Report
			if err := cbor.Unmarshal(incomingMessage.Options.SUITReports[i], &report); err != nil {
				t.logger.Printf("failed to Unmarshal SUIT Report: %v", err)
				continue
			}
			if report.Reference.Digest.DigestAlg != cose.AlgorithmSHA256 ||
				len(report.Reference.Digest.DigestBytes) != crypto.SHA256.Size() {
				// malformed
				t.logger.Printf("digest of a SUIT Report is malformed: %v", report.Reference.Digest)
				continue
			}
			manifestDigest, err := cbor.Marshal(report.Reference.Digest)
			if err != nil {
				t.logger.Printf("failed to encode a SUIT_Digest in SUIT_Report: %v", err)
			}
			if (report.Result.True != nil && *report.Result.True == true) ||
				(report.Result.DetailedResult != nil && report.Result.DetailedResult.ResultReason == suit.ReportReasonOK) {
				// record as success
				t.updateAgentStatusOnManifestError(agentKID, manifestDigest, incomingMessage.Options.SUITReports[i])
			}
		}

		// TODO: record other TEEP-level errors

		response = nil

	default:
		// should not reach here, because the corresponding sent message was valid
		return nil, ErrNotSupported
	}

	return response, nil
}

func (t *TAM) generateQueryRequest() ([]byte, error) {
	token := t.generateToken()
	if token == nil {
		return nil, ErrFatal
	}

	sendingQueryRequest := TEEPMessage{
		Type: TEEPTypeQueryRequest,
		Options: TEEPOptions{
			Token: token,
		},
		SupportedTEEPCipherSuites: util.DiagList[util.DiagList[TEEPCipherSuite]]{
			{
				{
					Type:      cose.CBORTagSign1Message,
					Algorithm: int(cose.AlgorithmESP256),
				},
			},
		},
		SupportedSUITCOSEProfiles: util.DiagList[suit.COSEProfile]{
			{
				DigestAlg:      cose.AlgorithmSHA256,
				AuthAlg:        cose.AlgorithmESP256,
				KeyExchangeAlg: cose.Algorithm(-29),
				EncryptionAlg:  cose.Algorithm(-65534),
			},
		},
		DataItemRequested: RequestDataItem(false, true, false, false),
	}
	response, err := sendingQueryRequest.COSESign1Sign(t.tamKey)
	if err != nil {
		return nil, ErrFatal
	}

	if err := t.saveSentQueryRequest(&sendingQueryRequest, nil); err != nil {
		return nil, ErrFatal
	}

	t.logger.Printf("token is saved: %s\n", hex.EncodeToString(token))
	return response, nil
}

func (t *TAM) generateQueryRequestWithAttestation() ([]byte, error) {
	challenge := t.generateChallenge()
	if challenge == nil {
		return nil, ErrFatal
	}

	sendingQueryRequest := TEEPMessage{
		Type: TEEPTypeQueryRequest,
		Options: TEEPOptions{
			Challenge: challenge,
		},
		SupportedTEEPCipherSuites: util.DiagList[util.DiagList[TEEPCipherSuite]]{
			{
				{
					Type:      cose.CBORTagSign1Message,
					Algorithm: int(cose.AlgorithmESP256),
				},
			},
		},
		SupportedSUITCOSEProfiles: util.DiagList[suit.COSEProfile]{
			{
				DigestAlg:      cose.AlgorithmSHA256,
				AuthAlg:        cose.AlgorithmESP256,
				KeyExchangeAlg: cose.Algorithm(-29),
				EncryptionAlg:  cose.Algorithm(-65534),
			},
		},
		// NOTE: request only attestation for simplicity
		DataItemRequested: RequestDataItem(true, false, false, false),
	}
	response, err := sendingQueryRequest.COSESign1Sign(t.tamKey)
	if err != nil {
		return nil, ErrFatal
	}

	if err := t.saveSentQueryRequest(&sendingQueryRequest, nil); err != nil {
		return nil, ErrFatal
	}

	t.logger.Printf("challenge is saved: %s\n", hex.EncodeToString(challenge))
	return response, nil
}

func verifyCOSESignature(raw []byte, pubKey *cose.Key) error {
	alg, err := pubKey.AlgorithmOrDefault()
	if err != nil {
		return fmt.Errorf("verifyCOSESignature: detect algorithm id: %w", err)
	}
	pub, err := pubKey.PublicKey()
	if err != nil {
		return fmt.Errorf("verifyCOSESignature: creating crypto.PublicKey: %w", err)
	}
	verifier, err := cose.NewVerifier(alg, pub)
	if err != nil {
		return fmt.Errorf("verifyCOSESignature: init verifier: %w", err)
	}

	// cose sign1
	var sign1 cose.Sign1Message
	if err := sign1.UnmarshalCBOR(raw); err == nil {
		if err := sign1.Verify(nil, verifier); err != nil {
			return fmt.Errorf("sign1 verification: %w", err)
		}
		return nil
	}
	// cose sign
	var sign cose.SignMessage
	if err := sign.UnmarshalCBOR(raw); err == nil {
		if err := sign.Verify(nil, verifier); err != nil {
			return fmt.Errorf("sign verification: %w", err)
		}
		return nil
	}

	return fmt.Errorf("verifyCOSESignature: unsupported COSE structure")
}

func (t *TAM) tryAuthenticateTeepMessage(raw []byte) (*TEEPMessage, []byte, error) {
	var sign1 cose.Sign1Message
	if err := sign1.UnmarshalCBOR(raw); err != nil {
		return nil, nil, err
	}
	message, kid, err := t.checkSignatureWithKID(sign1)
	if err != nil {
		t.logger.Print(err)
		return message, nil, err
	}
	return message, kid, nil
}

func (t *TAM) checkSignatureWithKID(msg cose.Sign1Message) (*TEEPMessage, []byte, error) {
	var teepMessage TEEPMessage
	var kid []byte
	if err := cbor.Unmarshal(msg.Payload, &teepMessage); err != nil {
		return nil, nil, ErrNotTEEPMessage
	}
	kid = getKid(msg.Headers.Unprotected)
	if kid == nil {
		return &teepMessage, nil, ErrKidIsMissing
	}

	key, err := t.getTEEPAgentKey(kid)
	if err != nil {
		return &teepMessage, nil, ErrNotAuthenticated
	}

	publicKey, err := key.PublicKey()
	if err != nil {
		return &teepMessage, nil, ErrFatal
	}
	alg, err := key.AlgorithmOrDefault()
	if err != nil {
		return &teepMessage, nil, ErrFatal
	}
	// create a verifier from a trusted private key
	verifier, err := cose.NewVerifier(alg, publicKey)
	if err != nil {
		return &teepMessage, nil, ErrFatal
	}

	err = msg.Verify(nil, verifier)
	if err != nil {
		return &teepMessage, nil, ErrNotAuthenticated
	}

	return &teepMessage, kid, nil
}

func getKid(u cose.UnprotectedHeader) []byte {
	t := u[int64(4)] // kid
	kid, ok := t.([]byte)
	if !ok || len(kid) == 0 {
		return nil
	}

	return kid
}

func (t *TAM) saveSentQueryRequest(sending *TEEPMessage, agentKID []byte) error {
	if sending == nil {
		return ErrFatal
	}
	if sending.Type != TEEPTypeQueryRequest {
		return ErrFatal
	}

	sentQueryRequestRepo := sqlite.NewSentQueryRequestMessageRepository(t.db)
	q := model.SentQueryRequestMessage{
		AgentID:              nil, // TODO, search Agent with KID
		AttestationRequested: sending.DataItemRequested.AttestationRequested(),
		TCListRequested:      sending.DataItemRequested.TCListRequested(),
	}
	if sending.Options.Token != nil {
		if _, err := sentQueryRequestRepo.CreateWithToken(t.ctx, sending.Options.Token, &q); err != nil {
			return ErrFatal
		}
	} else if sending.Options.Challenge != nil {
		if _, err := sentQueryRequestRepo.CreateWithChallenge(t.ctx, sending.Options.Challenge, &q); err != nil {
			return ErrFatal
		}
	} else {
		return ErrFatal
	}

	return nil
}

func (t *TAM) saveSentUpdate(sending *TEEPMessage, agentKID []byte, manifests []model.SuitManifest) error {
	if sending == nil {
		return ErrFatal
	}
	if sending.Type != TEEPTypeUpdate {
		return ErrFatal
	}

	q := model.SentUpdateMessageWithManifests{
		Token: model.Token{
			Token: sending.Options.Token,
		},
		Manifests: manifests,
	}

	sentUpdateRepo := sqlite.NewSentUpdateMessageRepository(t.db)
	if _, err := sentUpdateRepo.CreateWithToken(t.ctx, agentKID, sending.Options.Token, &q); err != nil {
		return ErrFatal
	}
	return nil
}

func (t *TAM) generateChallenge() []byte {
	challengeRepo := sqlite.NewChallengeRepository(t.db)
	if challengeRepo == nil {
		return nil
	}
	challenge, err := challengeRepo.GenerateUniqueChallenge(t.ctx)
	if err != nil {
		return nil
	}
	return challenge
}

// search sent TEEP message by the TAM itself with challenge
// returns the TEEPMessage, otherwise the error is set
func (t *TAM) searchSentMessageWithChallenge(challenge []byte) *TEEPMessage {
	if challenge == nil {
		return nil
	}
	challengeRepo := sqlite.NewChallengeRepository(t.db)
	if err := challengeRepo.MarkConsumed(t.ctx, challenge); err != nil {
		t.logger.Printf("failed to consume challenge %s: %v", hex.EncodeToString(challenge), err)
		return nil
	}

	sentQueryRequestRepo := sqlite.NewSentQueryRequestMessageRepository(t.db)
	if sentQueryRequestRepo == nil {
		return nil
	}
	sentQueryRequest, err := sentQueryRequestRepo.FindByChallenge(t.ctx, challenge)
	if err != nil {
		return nil
	}
	if sentQueryRequest != nil {
		sent := TEEPMessage{
			Type: TEEPTypeQueryRequest,
			Options: TEEPOptions{
				Challenge: challenge,
				// TODO items such as SupportedFreshnessMechanisms, ...
			},
			DataItemRequested: RequestDataItem(
				sentQueryRequest.SentQueryRequestMessage.AttestationRequested,
				sentQueryRequest.SentQueryRequestMessage.TCListRequested,
				false, // TODO
				false, // TODO
			),
		}
		return &sent
	}

	// nothing found
	return nil
}

func (t *TAM) generateToken() []byte {
	tokenRepo := sqlite.NewTokenRepository(t.db)
	if tokenRepo == nil {
		return nil
	}
	token, err := tokenRepo.GenerateUniqueToken(t.ctx)
	if err != nil {
		return nil
	}
	return token
}

// search sent TEEP message by the TAM itself
// returns the TEEPMessage, otherwise nil is returned
// (token is already consumed, token is not found, no message is bound to the token, ...)
func (t *TAM) searchSentMessageWithToken(token []byte) *TEEPMessage {
	if token == nil {
		return nil
	}
	tokenRepo := sqlite.NewTokenRepository(t.db)
	if err := tokenRepo.MarkConsumed(t.ctx, token); err != nil {
		t.logger.Printf("failed to consume token %s: %v", hex.EncodeToString(token), err)
		return nil
	}

	sentQueryRequestRepo := sqlite.NewSentQueryRequestMessageRepository(t.db)
	sentQueryRequest, err := sentQueryRequestRepo.FindByToken(t.ctx, token)
	if err != nil {
		return nil
	}
	if sentQueryRequest != nil {
		sent := TEEPMessage{
			Type: TEEPTypeQueryRequest,
			Options: TEEPOptions{
				Token: token,
				// TODO items such as SupportedFreshnessMechanisms, ...
			},
			DataItemRequested: RequestDataItem(
				sentQueryRequest.SentQueryRequestMessage.AttestationRequested,
				sentQueryRequest.SentQueryRequestMessage.TCListRequested,
				false, // TODO
				false, // TODO
			),
		}
		return &sent
	}

	sentUpdateRepo := sqlite.NewSentUpdateMessageRepository(t.db)
	if sentUpdateRepo == nil {
		return nil
	}
	sentUpdate, err := sentUpdateRepo.FindWithManifestsByToken(t.ctx, token)
	if err != nil {
		return nil
	}
	if sentUpdate != nil {
		var manifestList []util.BytesHexMax32
		for i := 0; i < len(sentUpdate.Manifests); i++ {
			manifestList = append(manifestList, sentUpdate.Manifests[i].Manifest)
		}
		sent := TEEPMessage{
			Type: TEEPTypeUpdate,
			Options: TEEPOptions{
				Token:        token,
				ManifestList: manifestList,
			},
		}
		return &sent
	}

	// nothing found
	return nil
}

// verifyAttestationPayload extracts the AttestationPayload from the provided TEEPMessage,
// sends it to the Verifier, and returns the resulting ProcessedAttestation.
// If any error occurs during processing, the returned ProcessedAttestation will be nil
// and an appropriate error will be returned.
func (t *TAM) verifyAttestationPayload(incoming *TEEPMessage) (*rats.ProcessedAttestation, error) {
	if incoming.Options.AttestationPayload == nil {
		return nil, ErrAttestationPayloadNotFound
	}

	result, err := t.submitAttestationPayload(incoming.Options.AttestationPayload)
	if err != nil {
		t.logger.Printf("failed to save attestation payload: %v", err)
		return nil, err
	}
	return result, nil
}

func (t *TAM) submitAttestationPayload(data []byte) (*rats.ProcessedAttestation, error) {
	if t.verifier == nil || (t.verifier != nil && reflect.ValueOf(t.verifier).IsNil()) {
		return nil, ErrVerifierNotConfigured
	}
	t.logger.Printf("submitting attestation payload to verifier: %v", t.verifier)

	att, err := t.verifier.Process(data)
	if err != nil {
		t.logger.Printf("challenge-response submission failed: %v", err)
		return nil, ErrAttestationFailed
	}

	return att, nil
}

func (t *TAM) processQueryResponse(incomingMessage *TEEPMessage, agentKID []byte, sentMessage *TEEPMessage) ([]byte, error) {
	// the incomingMessage (QueryResponse) must be authenticated and the sentMessage must be detected before this function
	if sentMessage == nil {
		return nil, ErrFatal
	}
	if !bytes.Equal(incomingMessage.Options.Token, sentMessage.Options.Token) {
		return nil, ErrFatal
	}

	token := t.generateToken()
	if token == nil {
		return nil, ErrFatal
	}
	t.logger.Printf("token is saved: %s\n", hex.EncodeToString(token))

	sendingUpdate := TEEPMessage{
		Type: TEEPTypeUpdate,
		Options: TEEPOptions{
			Token: token,
		},
	}

	// allow O(n^2) here because the n is expected small
	componentSet := make([][]byte, 0)
	// handle requested-tc-list
	for i := 0; i < len(incomingMessage.Options.RequestedTCList); i++ {
		requestedTC := incomingMessage.Options.RequestedTCList[i]
		encodedComponentID, err := cbor.Marshal(requestedTC.ComponentID)
		if err != nil {
			return nil, ErrFatal
		}
		j := 0
		for ; j < len(componentSet); j++ {
			if bytes.Equal(componentSet[j], encodedComponentID) {
				break
			}
		}
		if j >= len(componentSet) {
			componentSet = append(componentSet, encodedComponentID)
		}
	}
	// handle tc-list
	for i := 0; i < len(incomingMessage.Options.TCList); i++ {
		tc := incomingMessage.Options.TCList[i]
		encodedComponentID, err := cbor.Marshal(tc.SystemComponentID)
		if err != nil {
			return nil, ErrFatal
		}
		j := 0
		for ; j < len(componentSet); j++ {
			if bytes.Equal(componentSet[j], encodedComponentID) {
				break
			}
		}
		if j >= len(componentSet) {
			componentSet = append(componentSet, encodedComponentID)
		}
	}

	// TODO: add Trusted Component by the TAM decision

	if len(componentSet) == 0 {
		// ok, nothing should be installed or updated
		// replying empty message means session termination
		return nil, nil
	}

	// make manifest list
	sendingUpdate.Options.ManifestList = make([]util.BytesHexMax32, 0)
	manifests := make([]model.SuitManifest, 0)
	manifestRepo := sqlite.NewSuitManifestRepository(t.db)
	for i := 0; i < len(componentSet); i++ {
		manifest, err := manifestRepo.FindLatestByTrustedComponentID(t.ctx, componentSet[i])
		if err != nil {
			return nil, ErrFatal
		}
		if manifest == nil {
			t.logger.Printf("unknown SUIT Manifest is requested for ComponentID: %s", hex.EncodeToString(componentSet[i]))
		} else {
			sendingUpdate.Options.ManifestList = append(sendingUpdate.Options.ManifestList, manifest.Manifest)
			manifests = append(manifests, model.SuitManifest{ID: manifest.ID})
		}
	}

	if len(manifests) == 0 {
		// ok, no matching manifest found
		// replying empty message means session termination
		return nil, nil
	}

	// sign
	response, err := sendingUpdate.COSESign1Sign(t.tamKey)
	if err != nil {
		return nil, ErrFatal
	}

	if err := t.saveSentUpdate(&sendingUpdate, agentKID, manifests); err != nil {
		return nil, ErrFatal
	}

	return response, nil
}

// initializes the TAM by setting up database connections
func (t *TAM) InitDB(dbPath string) error {
	t.ctx = context.Background()
	// Initialize SQLite database (stored in tam_state.db or even could be :memory:)
	db, err := sqlite.InitDB(t.ctx, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	t.db = db

	return nil
}

func (t *TAM) SeedDemoData() error {
	if err := t.SeedDemoEntities(true); err != nil {
		return err
	}
	return t.SeedDemoAgent(true)
}

func (t *TAM) SeedDemoEntities(withManifest bool) error {
	refs, err := demo.SeedEntities(t.ctx, t.db, t.logger)
	if err != nil {
		return err
	}
	if !withManifest {
		return nil
	}
	return demo.SeedHelloTextManifest(t.ctx, t.db, refs.DeveloperSigningKeyID, t.logger)
}

func (t *TAM) SeedDemoAgent(withStatus bool) error {
	if err := t.ensureTrustedDemoAgentKey(); err != nil {
		return err
	}

	refs, err := demo.SeedEntities(t.ctx, t.db, t.logger)
	if err != nil {
		return err
	}
	return demo.SeedAgentScenario(t.ctx, t.db, refs.DeviceAdminID, t.logger, withStatus)
}

func (t *TAM) EnsureDefaultEntity(withManifest bool) error {
	return t.SeedDemoEntities(withManifest)
}

func (t *TAM) FindEntity(name string) (*model.Entity, error) {
	entityRepo := sqlite.NewEntityRepository(t.db)
	entity, err := entityRepo.FindByName(t.ctx, name)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (t *TAM) EnsureDefaultTEEPAgent(withStatus bool) error {
	return t.SeedDemoAgent(withStatus)
}

func (t *TAM) ensureTrustedDemoAgentKey() error {
	var key cose.Key
	if err := cbor.Unmarshal(demo.DefaultTrustedAgentKey, &key); err != nil {
		return errors.New("failed to initialize fixed TEEP Agent's key")
	}
	kid, err := key.Thumbprint(crypto.SHA256)
	if err != nil {
		return fmt.Errorf("derive demo trusted agent key kid: %w", err)
	}
	t.logger.Printf("Default TEEP Agent Key = %s, kid = %s", util.PrintCOSEKey(&key), util.BytesHexMax32(kid))
	existing, _ := t.getTEEPAgentKey(kid)
	if existing != nil {
		return nil
	}
	if err := t.setTEEPAgentKey(&key, nil); err != nil {
		return fmt.Errorf("store demo trusted agent key: %w", err)
	}
	t.logger.Printf("Stored default ESP256 TEEP Agent's key %s in system keyring.", hex.EncodeToString(kid))
	return nil
}

// Close closes the database connection.
func (t *TAM) Close() error {
	if t.db != nil {
		return sqlite.CloseDB(t.db)
	}
	return nil
}
