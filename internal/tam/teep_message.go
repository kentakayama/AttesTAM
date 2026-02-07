/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import (
	"crypto"
	"crypto/rand"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/veraison/go-cose"
)

// draft-ietf-teep-protocol

type TEEPMessage struct {
	_       struct{}        `cbor:",toarray"`
	Type    TEEPMessageType `cbor:"0,keyasint"`
	Options TEEPOptions     `cbor:"1,keyasint"`

	// for QueryRequest
	SupportedTEEPCipherSuites [][]TEEPCipherSuite
	SupportedSUITCOSEProfiles []suit.COSEProfile
	DataItemRequested         DataItemRequested

	// for Error
	ErrCode TEEPErrCode
}

func (m TEEPMessage) MarshalCBOR() ([]byte, error) {
	switch m.Type {
	case TEEPTypeQueryRequest:
		return cbor.Marshal([]any{m.Type, m.Options, m.SupportedTEEPCipherSuites, m.SupportedSUITCOSEProfiles, m.DataItemRequested})
	case TEEPTypeQueryResponse:
		return cbor.Marshal([]any{m.Type, m.Options})
	case TEEPTypeUpdate:
		return cbor.Marshal([]any{m.Type, m.Options})
	case TEEPTypeSuccess:
		return cbor.Marshal([]any{m.Type, m.Options})
	case TEEPTypeError:
		return cbor.Marshal([]any{m.Type, m.Options, m.ErrCode})
	default:
		return nil, ErrNotSupported
	}
}

func (m *TEEPMessage) UnmarshalCBOR(data []byte) error {
	var a []cbor.RawMessage
	err := cbor.Unmarshal(data, &a)
	if err != nil {
		return err
	}
	if len(a) < 2 {
		return ErrNotTEEPMessage
	}
	err = cbor.Unmarshal(a[0], &m.Type)
	if err != nil {
		return err
	}

	switch m.Type {
	case TEEPTypeQueryRequest:
		if len(a) != 5 {
			return ErrInvalidValue
		}
		err = cbor.Unmarshal(a[1], &m.Options)
		if err != nil {
			return err
		}
		err = cbor.Unmarshal(a[2], &m.SupportedTEEPCipherSuites)
		if err != nil {
			return err
		}
		err = cbor.Unmarshal(a[3], &m.SupportedSUITCOSEProfiles)
		if err != nil {
			return err
		}
		err = cbor.Unmarshal(a[4], &m.DataItemRequested)
		if err != nil {
			return err
		}
	case TEEPTypeQueryResponse, TEEPTypeUpdate, TEEPTypeSuccess:
		if len(a) != 2 {
			return ErrInvalidValue
		}
		err = cbor.Unmarshal(a[1], &m.Options)
		if err != nil {
			return err
		}
	case TEEPTypeError:
		if len(a) != 3 {
			return ErrInvalidValue
		}
		err = cbor.Unmarshal(a[1], &m.Options)
		if err != nil {
			return err
		}
		err = cbor.Unmarshal(a[2], &m.ErrCode)
		if err != nil {
			return err
		}
	default:
		return ErrNotSupported
	}

	return nil
}

// COSESign1Sign signs the TEEPMessage using the provided key and returns the signed message as a byte slice.
// It creates a signer from the key, generates the necessary headers, marshals the TEEPMessage into CBOR format,
// and then signs the message using COSE Sign1.
// The function returns the signed message and any error encountered during the process.
func (t *TEEPMessage) COSESign1Sign(key *cose.Key) ([]byte, error) {
	// create a signer
	signer, err := key.Signer()
	if err != nil {
		return nil, err
	}

	alg, err := key.AlgorithmOrDefault()
	if err != nil {
		return nil, err
	}
	kid, err := key.Thumbprint(crypto.SHA256)
	if err != nil {
		return nil, err
	}

	// create message header
	headers := cose.Headers{
		Protected: cose.ProtectedHeader{
			cose.HeaderLabelAlgorithm: alg,
		},
		Unprotected: cose.UnprotectedHeader{
			cose.HeaderLabelKeyID: kid,
		},
	}

	// encode TEEP Message
	tbsMessage, err := cbor.Marshal(t)
	if err != nil {
		return nil, err
	}

	// sign and marshal message
	return cose.Sign1(rand.Reader, signer, headers, tbsMessage, nil)
}

// COSESign1Verify verifies the signature of a COSE Sign1 message against the provided key.
// It takes a signed TEEP Protocol message byte slice and unmarshals it into a COSE Sign1 message.
// The function then verifies the signature using the verifier created from the key.
// If the verification is successful, it unmarshals the payload into the TEEPMessage instance.
// This function requires an existing TEEPMessage variable to store the result of the verification.
func (t *TEEPMessage) COSESign1Verify(key *cose.Key, sig []byte) error {
	// create a verifier
	verifier, err := key.Verifier()
	if err != nil {
		return err
	}

	// create a sign message from a raw COSE_Sign1 payload
	var msg cose.Sign1Message
	if err = msg.UnmarshalCBOR(sig); err != nil {
		return err
	}
	if err := msg.Verify(nil, verifier); err != nil {
		return err
	}

	return cbor.Unmarshal(msg.Payload, t)
}

type TEEPMessageType int

const (
	TEEPTypeUnknown       TEEPMessageType = 0
	TEEPTypeQueryRequest  TEEPMessageType = 1
	TEEPTypeQueryResponse TEEPMessageType = 2
	TEEPTypeUpdate        TEEPMessageType = 3
	TEEPTypeSuccess       TEEPMessageType = 5
	TEEPTypeError         TEEPMessageType = 6
)

func (t TEEPMessageType) String() string {
	switch t {
	case TEEPTypeQueryRequest:
		return "query-request"
	case TEEPTypeQueryResponse:
		return "query-response"
	case TEEPTypeUpdate:
		return "update"
	case TEEPTypeSuccess:
		return "teep-success"
	case TEEPTypeError:
		return "teep-error"
	default:
		return fmt.Sprintf("unknown(%d)", int(t))
	}
}

type TEEPOptions struct {
	SupportedTEEPCipherSuites    [][]TEEPCipherSuite         `cbor:"1,keyasint,omitempty"`
	Challenge                    []byte                      `cbor:"2,keyasint,omitempty"`
	Versions                     []TEEPVersion               `cbor:"3,keyasint,omitempty"`
	SupportedSUITCOSEProfiles    []suit.COSEProfile          `cbor:"4,keyasint,omitempty"`
	SelectedVersion              *TEEPVersion                `cbor:"6,keyasint,omitempty"`
	AttestationPayload           []byte                      `cbor:"7,keyasint,omitempty"`
	TCList                       []suit.SystemPropertyClaims `cbor:"8,keyasint,omitempty"`
	ExtList                      []TEEPExtInfo               `cbor:"9,keyasint,omitempty"`
	ManifestList                 []SUITManifestBstr          `cbor:"10,keyasint,omitempty"`
	Msg                          *string                     `cbor:"11,keyasint,omitempty"`
	ErrMsg                       *string                     `cbor:"12,keyasint,omitempty"`
	AttestationPayloadFormat     *string                     `cbor:"13,keyasint,omitempty"`
	RequestedTCList              []RequestedTCInfo           `cbor:"14,keyasint,omitempty"`
	UnneededManifestList         []SUITManifestBstr          `cbor:"15,keyasint,omitempty"`
	SUITReports                  [][]byte                    `cbor:"19,keyasint,omitempty"`
	Token                        []byte                      `cbor:"20,keyasint,omitempty"`
	SupportedFreshnessMechanisms []FreshnessMechanism        `cbor:"21,keyasint,omitempty"`
	ErrCode                      TEEPErrCode                 `cbor:"23,keyasint,omitempty"`
}

type TEEPVersion uint32
type TEEPExtInfo uint32

type DataItemRequested uint

const attestationMask = 0b1
const tcListMask = 0b10
const extensionsMask = 0b100
const suitReportsMask = 0b1000

func RequestDataItem(attestation bool, tcList bool, extensions bool, suitReports bool) DataItemRequested {
	var v DataItemRequested

	if attestation {
		v |= attestationMask
	}
	if tcList {
		v |= tcListMask
	}
	if extensions {
		v |= extensionsMask
	}
	if suitReports {
		v += suitReportsMask
	}
	return v
}

func (v DataItemRequested) AttestationRequested() bool {
	return v&attestationMask != 0
}

func (v DataItemRequested) TCListRequested() bool {
	return v&tcListMask != 0
}

func (v DataItemRequested) ExtensionsRequested() bool {
	return v&extensionsMask != 0
}

func (v DataItemRequested) SUITReportsRequested() bool {
	return v&suitReportsMask != 0
}

type TEEPErrCode uint8

const (
	TEEPErrPermanentError                 = 1
	TEEPErrUnsupportedExtension           = 2
	TEEPErrUnsupportedFreshnessMechanisms = 3
	TEEPErrUnsupportedMsgVersion          = 4
	TEEPErrUnsupportedCipherSuites        = 5
	TEEPErrBadCertificate                 = 6
	TEEPErrAttestationRequired            = 7
	TEEPErrUnsupportedSUITReport          = 8
	TEEPErrCertificateExpired             = 9
	TEEPErrTemporaryError                 = 10
	TEEPErrManifestProcessingFailed       = 17
)

func (e TEEPErrCode) String() string {
	switch e {
	case TEEPErrPermanentError:
		return "ERR_PERMANENT_ERROR"
	case TEEPErrUnsupportedExtension:
		return "ERR_UNSUPPORTED_EXTENSION"
	case TEEPErrUnsupportedFreshnessMechanisms:
		return "ERR_UNSUPPORTED_FRESHNESS_MECHANISMS"
	case TEEPErrUnsupportedMsgVersion:
		return "ERR_UNSUPPORTED_MSG_VERSION"
	case TEEPErrUnsupportedCipherSuites:
		return "ERR_UNSUPPORTED_CIPHER_SUITES"
	case TEEPErrBadCertificate:
		return "ERR_BAD_CERTIFICATE"
	case TEEPErrAttestationRequired:
		return "ERR_ATTESTATION_REQUIRED"
	case TEEPErrUnsupportedSUITReport:
		return "ERR_UNSUPPORTED_SUIT_REPORT"
	case TEEPErrCertificateExpired:
		return "ERR_CERTIFICATE_EXPIRED"
	case TEEPErrTemporaryError:
		return "ERR_TEMPORARY_ERROR"
	case TEEPErrManifestProcessingFailed:
		return "ERR_MANIFEST_PROCESSING_FAILED"
	default:
		return fmt.Sprintf("unknown(%d)", int(e))
	}
}

type TEEPCipherSuite struct {
	_         struct{} `cbor:",toarray"`
	Type      int      `cbor:"0,keyasint"`
	Algorithm int      `cbor:"1,keyasint"`
}

type RequestedTCInfo struct {
	ComponentID              suit.ComponentID `cbor:"16,keyasint,omitempty"`
	TCManifestSequenceNumber uint8            `cbor:"17,keyasint,omitempty"`
	HaveBinary               bool             `cbor:"18,keyasint,omitempty"`
}

type FreshnessMechanism uint

type SUITManifestBstr []byte
