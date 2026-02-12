/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
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
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/domain/model"
	"github.com/kentakayama/tam-over-http/internal/infra/rats"
	"github.com/kentakayama/tam-over-http/internal/infra/sqlite"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/kentakayama/tam-over-http/internal/util"
	"github.com/kentakayama/tam-over-http/resources"
	"github.com/veraison/eat"
	"github.com/veraison/go-cose"
)

type TAM struct {
	verifier    rats.IRAVerifier
	disableCOSE bool           // TODO: remove
	assets      responseAssets // TODO: remove
	logger      *log.Logger
	db          *sql.DB         // Database connection for TAM state
	ctx         context.Context // Background context for database operations
}

type responseAssets struct {
	queryCOSE   []byte
	queryPlain  []byte
	updateCOSE  []byte
	updatePlain []byte
	tamKey      *cose.Key
}

func NewTAM(disableCOSE bool, verifier rats.IRAVerifier, logger *log.Logger) (*TAM, error) {
	var key cose.Key
	err := cbor.Unmarshal(resources.TAMCoseKeyBytes, &key)
	if err != nil {
		return nil, errors.New("failed to load TAM's private key")
	}

	a := responseAssets{
		queryCOSE:   bytes.Clone(resources.QueryRequestCOSE),
		queryPlain:  bytes.Clone(resources.QueryRequestPlain),
		updateCOSE:  bytes.Clone(resources.UpdateCOSE),
		updatePlain: bytes.Clone(resources.UpdatePlain),
		tamKey:      &key,
	}
	if len(a.queryCOSE) == 0 {
		return nil, errors.New("missing embedded query request COSE payload")
	}
	if len(a.updateCOSE) == 0 {
		return nil, errors.New("missing embedded update COSE payload")
	}

	return &TAM{
		verifier:    verifier,
		disableCOSE: disableCOSE,
		assets:      a,
		logger:      logger,
	}, nil
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

	// TODO: search sent message by myself with token
	sentMessage := t.searchSentMessageWithToken(incomingMessage.Options.Token)
	// NOTE: sentMessage may be nil because the incomingMessage does not contain the token
	// case 1) TAM sent QueryRequest with challenge & request-attestation
	// case 2) someone created malformed TEEP Protocol messages

	t.logger.Printf("received TEEP Message %s.", incomingMessage.Type.CBORDiagString(0))

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
				// TODO: extract AttestationResult in EAT form, not Evidence
				// TODO: remove COSE_Sign support
				s, err := tryCOSESign1OrSign(incomingMessage.Options.AttestationPayload)
				if err != nil {
					return nil, ErrAttestationFailed
				}
				var rawAttestationPayload []byte
				switch m := any(s).(type) {
				case cose.Sign1Message:
					rawAttestationPayload = m.Payload
				case cose.SignMessage:
					rawAttestationPayload = m.Payload
				default:
					return nil, ErrNotSupported
				}

				var eat eat.Eat
				if err := eat.FromCBOR(rawAttestationPayload); err != nil {
					t.logger.Printf("failed to extract attestation public key: %v", err)
					return nil, ErrNotAuthenticated
				}

				// validate that the EAT payload is generated with the challenge the TAM sent
				if err := eat.Nonce.Validate(); err != nil {
					t.logger.Printf("failed to validate nonce: %v", err)
					return nil, ErrNotAuthenticated
				}
				if eat.Nonce.Len() != 1 {
					return nil, ErrNotAuthenticated
				}
				sentMessage = t.searchSentMessageWithChallenge(eat.Nonce.GetI(0))
				if sentMessage == nil {
					return nil, ErrNotAuthenticated
				}

				if eat.Cnf == nil || eat.Cnf.Key == nil {
					t.logger.Printf("attestation public key missing in payload")
					return nil, ErrNotAuthenticated
				}
				key := eat.Cnf.Key

				var ueid []byte
				if eat.UEID != nil && eat.UEID.Validate() != nil {
					ueid = []byte(*eat.UEID)
				}

				// verify QueryResponse signature
				if err := verifyCOSESignature(body, key); err != nil {
					t.logger.Printf("query-response verification failed: %v", err)
					return nil, ErrNotAuthenticated
				}

				// store the public key for future use
				if err := t.setTEEPAgentKey(key, ueid); err != nil {
					t.logger.Printf("failed to store attestation public key: %v", err)
					return nil, ErrFatal
				} else {
					t.logger.Printf("stored attestation public key %v in system keyring.", key)
				}
				agentKID, _ = key.Thumbprint(crypto.SHA256)
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
	response, err := sendingQueryRequest.COSESign1Sign(t.assets.tamKey)
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
	response, err := sendingQueryRequest.COSESign1Sign(t.assets.tamKey)
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
	s, err := tryCOSESign1OrSign(raw)
	if err != nil {
		return nil, nil, err
	}
	message, kid, err := t.checkSignature(s)
	if err != nil {
		t.logger.Print(err)
		return message, nil, err
	}
	return message, kid, nil
}

func tryCOSESign1OrSign(raw []byte) (any, error) {
	// Try COSE_Sign1
	var sign1 cose.Sign1Message
	if err := sign1.UnmarshalCBOR(raw); err == nil {
		return sign1, nil
	}

	// Try COSE_Sign
	var sign cose.SignMessage
	if err := sign.UnmarshalCBOR(raw); err == nil {
		return sign, nil
	}

	return nil, ErrNotSupported
}

func (t *TAM) checkSignature(msg any) (*TEEPMessage, []byte, error) {
	var teepMessage TEEPMessage
	var kid []byte
	switch m := msg.(type) {
	case cose.Sign1Message:
		err := cbor.Unmarshal(m.Payload, &teepMessage)
		if err != nil {
			return nil, nil, ErrNotTEEPMessage
		}
		kid = getKid(m.Headers.Unprotected)
		if kid == nil {
			return &teepMessage, nil, ErrKidIsMissing
		}
	case cose.SignMessage:
		err := cbor.Unmarshal(m.Payload, &teepMessage)
		if err != nil {
			return nil, nil, ErrNotTEEPMessage
		}
		kid = getKid(m.Headers.Unprotected)
		if kid == nil {
			return &teepMessage, nil, ErrKidIsMissing
		}
	default:
		return nil, nil, ErrNotSupported
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

	switch m := msg.(type) {
	case cose.Sign1Message:
		err = m.Verify(nil, verifier)
		if err != nil {
			return &teepMessage, nil, ErrNotAuthenticated
		}
	case cose.SignMessage:
		err = m.Verify(nil, verifier)
		if err != nil {
			return &teepMessage, nil, ErrNotAuthenticated
		}
	default:
		return &teepMessage, nil, ErrNotSupported
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
	if t.verifier == nil {
		return nil, ErrAttestationFailed
	}

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

	t.logger.Printf("QueryResponse payload (COSE CBOR):\n%#v", *incomingMessage)

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

	// sign
	response, err := sendingUpdate.COSESign1Sign(t.assets.tamKey)
	if err != nil {
		return nil, ErrFatal
	}

	if err := t.saveSentUpdate(&sendingUpdate, agentKID, manifests); err != nil {
		return nil, ErrFatal
	}

	return response, nil
}

// Init initializes the TAM by setting up database connections and loading/creating default entities.
func (t *TAM) Init() error {
	return t.InitWithPath("tam_state.db")
}

func (t *TAM) InitWithPath(dbPath string) error {
	t.ctx = context.Background()
	// Initialize SQLite database (stored in tam_state.db or even could be :memory:)
	db, err := sqlite.InitDB(t.ctx, dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	t.db = db

	return nil
}

func (t *TAM) EnsureDefaultEntity(withManifest bool) error {
	// XXX: initialize default entiries only for demo purpose

	// Try to find or create "default" entities
	entityRepo := sqlite.NewEntityRepository(t.db)

	// add default TAM admin if not exists
	defaultTAMAdmin := &model.Entity{
		Name:       "admin@example.com",
		IsTAMAdmin: true,
		CreatedAt:  time.Now().UTC(),
	}
	var admID int64
	adm, err := entityRepo.FindByName(t.ctx, defaultTAMAdmin.Name)
	if err != nil {
		return fmt.Errorf("failed to find default TAM Admin: %w", err)
	}
	if adm != nil {
		admID = adm.ID
		// OK, already exists
	} else {
		admID, err = entityRepo.Create(t.ctx, defaultTAMAdmin)
		if err != nil {
			return fmt.Errorf("failed to create default TAM Admin: %w", err)
		}
		t.logger.Printf("Created default TAM Admin with ID: %d", admID)
	}

	// Create manifest signing key
	var adminKeyID int64
	adminKID := []byte("key-1")
	keyRepo := sqlite.NewManifestSigningKeyRepository(t.db)
	adminKey, err := keyRepo.FindByKID(t.ctx, adminKID)
	if err != nil || adminKey == nil {
		// create the key
		key := &model.ManifestSigningKey{
			KID:       []byte("key-1"),
			EntityID:  admID,
			PublicKey: []byte("pub-key-1"),
		}
		k, err := keyRepo.Create(t.ctx, key)
		if err != nil {
			t.logger.Printf("create manifest signing key error: %v", err)
		} else {
			adminKeyID = k
			t.logger.Printf("Created default TAM Admin key with ID: %d", adminKeyID)
		}
	} else {
		adminKeyID = adminKey.ID
	}

	// add default developer if not exists
	defaultDev := &model.Entity{
		Name:          "developer1@example.com",
		IsTCDeveloper: true,
		CreatedAt:     time.Now().UTC(),
	}

	var devID int64
	dev, err := entityRepo.FindByName(t.ctx, defaultDev.Name)
	if err != nil {
		return fmt.Errorf("failed to find default TC Developer: %w", err)
	}
	if dev != nil {
		devID = dev.ID
		// OK, already exists
	} else {
		devID, err = entityRepo.Create(t.ctx, defaultDev)
		if err != nil {
			return fmt.Errorf("failed to create default TC Developer: %w", err)
		}
		t.logger.Printf("Created default TC Developer with ID: %d", devID)
	}

	// add default developer key if not exists
	developerKey := cose.Key{
		Type:      cose.KeyTypeEC2,
		Algorithm: cose.AlgorithmESP256,
		Params: map[any]any{
			cose.KeyLabelEC2Curve: cose.CurveP256,
			cose.KeyLabelEC2X: []byte{
				0x84, 0x96, 0x81, 0x1A, 0xAE, 0x0B, 0xAA, 0xAB,
				0xD2, 0x61, 0x57, 0x18, 0x9E, 0xEC, 0xDA, 0x26,
				0xBE, 0xAA, 0x8B, 0xF1, 0x1B, 0x6F, 0x3F, 0xE6,
				0xE2, 0xB5, 0x65, 0x9C, 0x85, 0xDB, 0xC0, 0xAD,
			},
			cose.KeyLabelEC2Y: []byte{
				0x3B, 0x1F, 0x2A, 0x4B, 0x6C, 0x09, 0x81, 0x31,
				0xC0, 0xA3, 0x6D, 0xAC, 0xD1, 0xD7, 0x8B, 0xD3,
				0x81, 0xDC, 0xDF, 0xB0, 0x9C, 0x05, 0x2D, 0xB3,
				0x39, 0x91, 0xDB, 0x73, 0x38, 0xB4, 0xA8, 0x96,
			},
		},
	}
	kid, err := developerKey.Thumbprint(crypto.SHA256)
	if err != nil {
		return nil
	}
	encodedKey, err := cbor.Marshal(developerKey)
	if err != nil {
		return nil
	}

	defaultDevKey := &model.ManifestSigningKey{
		KID:       kid,
		EntityID:  devID,
		PublicKey: encodedKey,
	}
	var devKeyID int64
	k, err := keyRepo.FindByKID(t.ctx, kid)
	if err != nil {
		return fmt.Errorf("failed to find default TC Developer: %w", err)
	}
	if k != nil {
		// OK, already exists
		devKeyID = k.ID
	} else {
		devKeyID, err = keyRepo.Create(t.ctx, defaultDevKey)
		if err != nil {
			return fmt.Errorf("failed to create default TC Developer: %w", err)
		}
		t.logger.Printf("Created default TC Developer with ID: %d", devKeyID)
	}

	// add default device manager admin if not exists
	defaultDevAdmin := &model.Entity{
		Name:          "dev-admin1@example.com",
		IsDeviceAdmin: true,
		CreatedAt:     time.Now().UTC(),
	}

	var devAdminID int64
	devAdmin, err := entityRepo.FindByName(t.ctx, defaultDevAdmin.Name)
	if err != nil {
		return fmt.Errorf("failed to find default TC Developer: %w", err)
	}
	if devAdmin != nil {
		devAdminID = devAdmin.ID
		// OK, already exists
	} else {
		devAdminID, err = entityRepo.Create(t.ctx, defaultDevAdmin)
		if err != nil {
			return fmt.Errorf("failed to create default Device Admin: %w", err)
		}
		t.logger.Printf("Created default Device Admin: %d", devAdminID)
	}

	if withManifest {
		manifestRepo := sqlite.NewSuitManifestRepository(t.db)
		trusted1 := []byte{0x81, 0x49, 0x61, 0x70, 0x70, 0x31, 0x2E, 0x77, 0x61, 0x73, 0x6D} // ['app1.wasm']
		digestM1 := []byte("digest1")
		m1 := &model.SuitManifest{Manifest: []byte("m1"), Digest: digestM1, SigningKeyID: devKeyID, TrustedComponentID: trusted1, SequenceNumber: 3}
		_, err = manifestRepo.Create(t.ctx, m1)
		if err != nil {
			t.logger.Printf("create manifest m1 error: %v", err)
		}
		trusted2 := []byte{0x81, 0x49, 0x61, 0x70, 0x70, 0x32, 0x2E, 0x77, 0x61, 0x73, 0x6D} // ['app2.wasm']
		digestM2 := []byte("digest2")
		m2 := &model.SuitManifest{Manifest: []byte("m2"), Digest: digestM2, SigningKeyID: devKeyID, TrustedComponentID: trusted2, SequenceNumber: 2}
		_, err = manifestRepo.Create(t.ctx, m2)
		if err != nil {
			t.logger.Printf("create manifest m2 error: %v", err)
		}

		tcID := []byte{
			0x81,                                                       // [
			0x49, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, // 'hello.txt'
		}
		// from internal/suit/manifest_test.go
		taggedManifest0 := []byte{
			0xd8, 0x6b, // 107(
			0xa3,             // {
			0x02, 0x58, 0x96, // authentication-wrapper: <<
			0x82, 0x58, 0x24, // [ <<
			0x82,       // [
			0x2f,       // -16 / :SHA256 /
			0x58, 0x20, // h'
			0x43, 0x13, 0x16, 0x04, 0x84, 0x18, 0x2f, 0x04, 0x11, 0x97, 0xf6, 0x95, 0xa4, 0x12, 0xb7, 0xc5,
			0x91, 0xcb, 0x11, 0x2c, 0xca, 0xaa, 0x5d, 0x60, 0xc0, 0x32, 0x85, 0xef, 0x7e, 0x20, 0xfc, 0xb0,

			0x58, 0x6d, 0xd2, // << 18(
			0x84,                   // [
			0x43, 0xa1, 0x01, 0x28, // << { / alg / 1: -9 / ESP256 / } >>
			0xa1, 0x04, 0x58, 0x20, // { / kid / 4: h'
			0xca, 0x9e, 0x35, 0xf2, 0x3b, 0x2b, 0x52, 0x5f, 0xb4, 0xfc, 0x83, 0xf5, 0x12, 0xb0, 0xdc, 0xac,
			0x4a, 0xc2, 0x9e, 0x45, 0x7e, 0x87, 0x3a, 0x5d, 0x6a, 0x73, 0x13, 0xf7, 0x16, 0x90, 0xb3, 0x3c,
			0xf6, // null
			0x58, 0x40,
			0x10, 0xab, 0x19, 0x47, 0x96, 0x8d, 0x60, 0x6e, 0x98, 0xb3, 0xd2, 0x26, 0x75, 0xe5, 0x9c, 0x71,
			0x62, 0x44, 0x27, 0x27, 0x5f, 0xcd, 0x98, 0xcc, 0xa1, 0x54, 0x14, 0x4d, 0x0f, 0x51, 0xff, 0x52,
			0xfb, 0xd9, 0x58, 0xbe, 0xbc, 0xc3, 0x30, 0xd0, 0xcf, 0xb2, 0xb6, 0x05, 0x31, 0xfa, 0x7a, 0x46,
			0x2b, 0x57, 0x76, 0xda, 0x1e, 0xc1, 0xde, 0x94, 0xf9, 0xe1, 0x38, 0x31, 0x5d, 0xd2, 0x54, 0x19,

			0x03, 0x58, 0x99, 0xa5, // manifest: << {
			0x01, // manifest-version
			0x01,
			0x02, // manifest-sequence-number
			0x00,
			0x03, // common
			0x58, 0x65, 0xa2, 0x02,
			0x81, 0x81, // [ [
			0x49, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, // 'hello.txt'
			0x04,             // shared-sequence
			0x58, 0x54, 0x86, // << [
			0x14, 0xa4, // override-parameters: {
			0x01, 0x50, 0xfa, 0x6b, 0x4a, 0x53, 0xd5, 0xad, 0x5f, 0xdf, 0xbe, 0x9d, 0xe6, 0x63, 0xe4, 0xd4, 0x1f, 0xfe,
			0x02, 0x50, 0x14, 0x92, 0xaf, 0x14, 0x25, 0x69, 0x5e, 0x48, 0xbf, 0x42, 0x9b, 0x2d, 0x51, 0xf2, 0xab, 0x45,
			0x03, 0x58, 0x24, 0x82, // image-digest: << [
			0x2f, // -16: SHA256
			0x58, 0x20,
			0xdf, 0xfd, 0x60, 0x21, 0xbb, 0x2b, 0xd5, 0xb0, 0xaf, 0x67, 0x62, 0x90, 0x80, 0x9e, 0xc3, 0xa5,
			0x31, 0x91, 0xdd, 0x81, 0xc7, 0xf7, 0x0a, 0x4b, 0x28, 0x68, 0x8a, 0x36, 0x21, 0x82, 0x98, 0x6f,
			0x0e, 0x0d, // image-size: 13}
			0x01, 0x0f, // suit-condition-vendor-identifier
			0x02, 0x0f, // suit-condition-class-identifier
			0x05,                                                                                                                               // manifest-component-id:
			0x81, 0x54, 0x6d, 0x61, 0x6e, 0x69, 0x66, 0x65, 0x73, 0x74, 0x2e, 0x74, 0x65, 0x78, 0x74, 0x2e, 0x30, 0x2e, 0x73, 0x75, 0x69, 0x74, // 'manifest.text.0.suit'
			0x10,       // payload-fetch
			0x53, 0x86, // << [
			0x14, 0xa1, // override-parameters: {
			0x15, 0x6a, 0x23, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, // uri: "#hello.txt"}
			0x15, 0x02,
			0x03, 0x0f,

			0x6a, 0x23, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, // "#hello.txt":
			0x4d, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21, // "Hello, World!"
		}
		defaultManifest0 := &model.SuitManifest{
			Manifest: taggedManifest0,
			Digest: []byte{
				0x82,       // [
				0x2f,       // -16 / :SHA256 /
				0x58, 0x20, // h'
				0x43, 0x13, 0x16, 0x04, 0x84, 0x18, 0x2f, 0x04, 0x11, 0x97, 0xf6, 0x95, 0xa4, 0x12, 0xb7, 0xc5,
				0x91, 0xcb, 0x11, 0x2c, 0xca, 0xaa, 0x5d, 0x60, 0xc0, 0x32, 0x85, 0xef, 0x7e, 0x20, 0xfc, 0xb0,
			},
			SigningKeyID:       devKeyID,
			TrustedComponentID: tcID,
			SequenceNumber:     0,
		}

		m, err := manifestRepo.FindLatestByTrustedComponentID(t.ctx, tcID)
		if err != nil {
			return fmt.Errorf("failed to find default TC Manifest: %w", err)
		}
		if m != nil && m.SequenceNumber == defaultManifest0.SequenceNumber {
			// OK, already exists
		} else {
			mID, err := manifestRepo.Create(t.ctx, defaultManifest0)
			if err != nil {
				return fmt.Errorf("failed to create default TC Manifest: %w", err)
			}
			t.logger.Printf("Created default TC Manifest for TC %s with ID: %d", hex.EncodeToString(tcID), mID)
		}

		untaggedManifest1 := []byte{
			0xa3, 0x02, 0x58, 0x96, 0x82, 0x58, 0x24, 0x82, 0x2f, 0x58, 0x20, 0xb2,
			0xa0, 0x0e, 0x3e, 0x70, 0x7a, 0x11, 0x7f, 0x73, 0x0a, 0x08, 0x77, 0x9a,
			0x1e, 0xba, 0x26, 0x41, 0x3f, 0x00, 0x5d, 0xb3, 0x8d, 0x01, 0x11, 0xbb,
			0xa9, 0xc0, 0x5d, 0x1f, 0x40, 0x27, 0xc0, 0x58, 0x6d, 0xd2, 0x84, 0x43,
			0xa1, 0x01, 0x28, 0xa1, 0x04, 0x58, 0x20, 0xca, 0x9e, 0x35, 0xf2, 0x3b,
			0x2b, 0x52, 0x5f, 0xb4, 0xfc, 0x83, 0xf5, 0x12, 0xb0, 0xdc, 0xac, 0x4a,
			0xc2, 0x9e, 0x45, 0x7e, 0x87, 0x3a, 0x5d, 0x6a, 0x73, 0x13, 0xf7, 0x16,
			0x90, 0xb3, 0x3c, 0xf6, 0x58, 0x40, 0xd7, 0xda, 0x88, 0x43, 0x40, 0x15,
			0x93, 0x94, 0xce, 0xb6, 0x31, 0x47, 0xfa, 0xc9, 0x2f, 0xd6, 0x24, 0xca,
			0xa6, 0x39, 0x01, 0xce, 0x39, 0xc2, 0x02, 0x7b, 0xd9, 0xd1, 0xc1, 0xdb,
			0x80, 0x54, 0x1a, 0x97, 0x45, 0x76, 0x8a, 0x8c, 0xbf, 0x17, 0x7a, 0x91,
			0x67, 0xba, 0x5b, 0xb8, 0x76, 0x65, 0xc7, 0xc7, 0xf1, 0x4d, 0xb4, 0x32,
			0x30, 0xba, 0x0c, 0x67, 0x5d, 0xa4, 0xe6, 0x2d, 0xe0, 0xe5, 0x03, 0x58,
			0x99, 0xa5, 0x01, 0x01, 0x02, 0x01, 0x03, 0x58, 0x65, 0xa2, 0x02, 0x81,
			0x81, 0x49, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, 0x04,
			0x58, 0x54, 0x86, 0x14, 0xa4, 0x01, 0x50, 0xfa, 0x6b, 0x4a, 0x53, 0xd5,
			0xad, 0x5f, 0xdf, 0xbe, 0x9d, 0xe6, 0x63, 0xe4, 0xd4, 0x1f, 0xfe, 0x02,
			0x50, 0x14, 0x92, 0xaf, 0x14, 0x25, 0x69, 0x5e, 0x48, 0xbf, 0x42, 0x9b,
			0x2d, 0x51, 0xf2, 0xab, 0x45, 0x03, 0x58, 0x24, 0x82, 0x2f, 0x58, 0x20,
			0xc2, 0xfa, 0xe0, 0xb4, 0x5d, 0xc6, 0xb4, 0x57, 0xad, 0xeb, 0xce, 0x23,
			0x68, 0xbb, 0x8c, 0x2e, 0xba, 0xd8, 0x18, 0xfd, 0xd7, 0xbe, 0xb7, 0xc8,
			0xac, 0x77, 0xf2, 0x29, 0x5e, 0x69, 0x94, 0x81, 0x0e, 0x15, 0x01, 0x0f,
			0x02, 0x0f, 0x05, 0x81, 0x54, 0x6d, 0x61, 0x6e, 0x69, 0x66, 0x65, 0x73,
			0x74, 0x2e, 0x74, 0x65, 0x78, 0x74, 0x2e, 0x31, 0x2e, 0x73, 0x75, 0x69,
			0x74, 0x10, 0x53, 0x86, 0x14, 0xa1, 0x15, 0x6a, 0x23, 0x68, 0x65, 0x6c,
			0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, 0x15, 0x02, 0x03, 0x0f, 0x6a, 0x23,
			0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e, 0x74, 0x78, 0x74, 0x55, 0x48, 0x65,
			0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64,
			0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21,
		}
		defaultManifest1 := &model.SuitManifest{
			Manifest: untaggedManifest1,
			Digest: []byte{
				0x82, 0x2f, 0x58, 0x20,
				0xb2, 0xa0, 0x0e, 0x3e, 0x70, 0x7a, 0x11, 0x7f, 0x73, 0x0a, 0x08, 0x77, 0x9a, 0x1e, 0xba, 0x26,
				0x41, 0x3f, 0x00, 0x5d, 0xb3, 0x8d, 0x01, 0x11, 0xbb, 0xa9, 0xc0, 0x5d, 0x1f, 0x40, 0x27, 0xc0,
			},
			SigningKeyID:       devKeyID,
			TrustedComponentID: tcID,
			SequenceNumber:     1,
		}
		m, err = manifestRepo.FindLatestByTrustedComponentID(t.ctx, tcID)
		if err != nil {
			return fmt.Errorf("failed to find default TC Manifest: %w", err)
		}
		if m != nil && m.SequenceNumber == defaultManifest1.SequenceNumber {
			// OK, already exists
		} else {
			mID, err := manifestRepo.Create(t.ctx, defaultManifest1)
			if err != nil {
				return fmt.Errorf("failed to create default TC Manifest: %w", err)
			}
			t.logger.Printf("Created default TC Manifest for TC %s with ID: %d", hex.EncodeToString(tcID), mID)
		}
	}

	return nil
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
	// XXX: initialize default entiries only for demo purpose

	fixedESP256AgentKey := []byte{
		0xA6,       //# map(6)
		0x01,       //# unsigned(1) / 1 = kty /
		0x02,       //# unsigned(2) / 2 = EC2 /
		0x03,       //# unsigned(3) / 3 = alg /
		0x28,       //# negative(8) / -9 = ESP256 /
		0x20,       //# negative(0) / -1 = crv /
		0x01,       //# unsigned(1) / 1 = P-256 /
		0x21,       //# negative(1) / -2 = x /
		0x58, 0x20, //# bytes(32)
		0xBE, 0x7C, 0x56, 0x99, 0x3F, 0x71, 0x11, 0x45,
		0x34, 0xC2, 0xF4, 0xA4, 0xF4, 0xE4, 0x60, 0x67,
		0x84, 0xFA, 0x9D, 0x96, 0x35, 0xE1, 0x22, 0xBC,
		0x8A, 0x49, 0x0B, 0x2E, 0x11, 0xFE, 0xB9, 0x32,
		0x22,       //# negative(2) / -3 = y /
		0x58, 0x20, //# bytes(32)
		0x81, 0x69, 0x6B, 0x42, 0xC3, 0xBE, 0x1B, 0x24,
		0x4C, 0xC0, 0x3B, 0xCA, 0x97, 0xF0, 0xCE, 0x75,
		0xE2, 0xD9, 0x3A, 0xDA, 0x1C, 0xE5, 0x56, 0x62,
		0x92, 0x27, 0xF1, 0x0A, 0x8C, 0x2C, 0x5B, 0x29,
		0x23,       //# negative(3) / -4 = d /
		0x58, 0x20, //# bytes(32)
		0xA1, 0x3D, 0x1C, 0x9F, 0x42, 0x78, 0x04, 0x70,
		0x82, 0xC4, 0xA4, 0x06, 0xEF, 0x33, 0xA9, 0xAE,
		0xD2, 0xDA, 0x01, 0x05, 0x87, 0xA3, 0x75, 0x1E,
		0xAB, 0xAA, 0x0B, 0x6B, 0xA0, 0x12, 0x63, 0xE3,
	}
	var keyESP256 cose.Key
	if err := cbor.Unmarshal(fixedESP256AgentKey, &keyESP256); err != nil {
		return errors.New("failed to initialize fixed TEEP Agent's key")
	}
	kidESP256, _ := keyESP256.Thumbprint(crypto.SHA256)
	t.logger.Printf("ESP256 Key = {x: %s, y: %s, kid: %s}",
		hex.EncodeToString(keyESP256.Params[cose.KeyLabelEC2X].([]byte)),
		hex.EncodeToString(keyESP256.Params[cose.KeyLabelEC2Y].([]byte)),
		hex.EncodeToString(kidESP256))
	agentESP256, _ := t.getTEEPAgentKey(kidESP256)
	if agentESP256 != nil {
		// OK, already exists
	} else {
		if err := t.setTEEPAgentKey(&keyESP256, nil); err != nil {
			// log the error but do not fail initialization
			t.logger.Printf("Failed to store default ESP256 TEEP Agent's key: %v", err)
		} else {
			t.logger.Printf("Stored default ESP256 TEEP Agent's key %s in system keyring.", hex.EncodeToString(kidESP256))
		}
	}

	if withStatus {
		now := time.Now().UTC().Truncate(time.Second)

		// search the default device admin
		adminName := "dev-admin1@example.com"
		entityRepo := sqlite.NewEntityRepository(t.db)
		admin, err := entityRepo.FindByName(t.ctx, adminName)
		if err != nil || admin == nil {
			t.logger.Fatal("failed to find admin: %w", err)
			return nil
		}

		// Create a device directly and get its ID
		var deviceID *int64
		ueid := append([]byte{0x01}, []byte("building-dev-123")...)
		deviceRepo := sqlite.NewDeviceRepository(t.db)
		device, err := deviceRepo.FindByUEID(t.ctx, ueid)
		if err != nil || device == nil {
			// dummy random UEID with 128-bit field and 1 byte prefix
			deviceModel := &model.Device{UEID: ueid, AdminID: admin.ID, CreatedAt: now}
			deviceIDVal, err := deviceRepo.Create(t.ctx, deviceModel)
			if err != nil {
				// can be ignored
				t.logger.Printf("insert device error: %v", err)
			} else {
				deviceID = &deviceIDVal
			}
		}

		// create an agent
		agentKID := []byte("dummy-teep-agent-kid-for-dev-123")
		agentRepo := sqlite.NewAgentRepository(t.db)
		agent, err := agentRepo.FindByKID(t.ctx, agentKID)
		if err != nil || agent == nil {
			agentModel := &model.Agent{KID: agentKID, DeviceID: deviceID, PublicKey: []byte("pk"), CreatedAt: now, ExpiredAt: now.Add(30 * 365 * 24 * time.Hour)}
			_, err = agentRepo.Create(t.ctx, agentModel)
			if err != nil {
				t.logger.Printf("create agent error: %v", err)
			}
		}

		// create an agent status
		agentStatusRepo := sqlite.NewAgentStatusRepository(t.db)
		agentStatus, err := agentStatusRepo.GetAgentStatus(t.ctx, agentKID)
		if err != nil || agentStatus == nil {
			t.logger.Printf("failed to find agent status: %v", err)
		} else {
			digestM1 := []byte("digest1")
			report1 := []byte("report1")
			err = agentStatusRepo.ReflectManifestSuccess(t.ctx, agentKID, digestM1, report1)
			if err != nil {
				t.logger.Printf("create agent status 1 error: %v", err)
			}

			digestM2 := []byte("digest2")
			report2 := []byte("report2")
			err = agentStatusRepo.ReflectManifestSuccess(t.ctx, agentKID, digestM2, report2)
			if err != nil {
				t.logger.Printf("create agent status 2 error: %v", err)
			}
		}
	}

	return nil
}

// Close closes the database connection.
func (t *TAM) Close() error {
	if t.db != nil {
		return sqlite.CloseDB(t.db)
	}
	return nil
}
