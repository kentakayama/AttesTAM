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
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/kentakayama/tam-over-http/internal/suit"
	"github.com/kentakayama/tam-over-http/internal/util"
	"github.com/veraison/go-cose"
)

// draft-ietf-teep-protocol

type TEEPMessage struct {
	_       struct{}        `cbor:",toarray"`
	Type    TEEPMessageType `cbor:"0,keyasint"`
	Options TEEPOptions     `cbor:"1,keyasint"`

	// for QueryRequest
	SupportedTEEPCipherSuites util.DiagList[util.DiagList[TEEPCipherSuite]]
	SupportedSUITCOSEProfiles util.DiagList[suit.COSEProfile]
	DataItemRequested         DataItemRequested

	// for Error
	ErrCode TEEPErrCode
}

func (m TEEPMessage) String() string {
	return m.CBORDiagString(0)
}

func (m TEEPMessage) CBORDiagString(indent int) string {
	pad1 := strings.Repeat("  ", indent)
	pad2 := strings.Repeat("  ", indent+1)
	var encodedStrings []string
	encodedStrings = append(encodedStrings, fmt.Sprintf("/ TEEPMessage: / [\n%s/ type: / %s", pad2, m.Type.CBORDiagString(0)))
	encodedStrings = append(encodedStrings, fmt.Sprintf("%s/ options: / %s", pad2, m.Options.CBORDiagString(indent+1)))
	switch m.Type {
	case TEEPTypeQueryRequest:
		encodedStrings = append(encodedStrings, fmt.Sprintf("%s/ supported-teep-cipher-suites: / %v", pad2, m.SupportedTEEPCipherSuites.CBORDiagString(0)))
		encodedStrings = append(encodedStrings, fmt.Sprintf("%s/ supported-suit-cose-profiles: / %v", pad2, m.SupportedSUITCOSEProfiles.CBORDiagString(0)))
		encodedStrings = append(encodedStrings, fmt.Sprintf("%s/ data-item-requested: / %v", pad2, m.DataItemRequested.CBORDiagString(0)))
	case TEEPTypeError:
		encodedStrings = append(encodedStrings, fmt.Sprintf("%s/ err-code: / %s", pad2, m.ErrCode.CBORDiagString(0)))
	}
	return strings.Join(encodedStrings, ",\n") + "\n" + pad1 + "]"
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

func (t TEEPMessageType) CBORDiagString(indent int) string {
	switch t {
	case TEEPTypeQueryRequest:
		return "1 / query-request /"
	case TEEPTypeQueryResponse:
		return "2 / query-response /"
	case TEEPTypeUpdate:
		return "3 / update /"
	case TEEPTypeSuccess:
		return "5 / success /"
	case TEEPTypeError:
		return "6 / error /"
	default:
		return fmt.Sprintf("unknown(%d)", int(t))
	}
}

type TEEPOptions struct {
	SupportedTEEPCipherSuites    util.DiagList[util.DiagList[TEEPCipherSuite]] `cbor:"1,keyasint,omitempty"`
	Challenge                    util.BytesHexMax32                            `cbor:"2,keyasint,omitempty"`
	Versions                     util.DiagList[TEEPVersion]                    `cbor:"3,keyasint,omitempty"`
	SupportedSUITCOSEProfiles    util.DiagList[suit.COSEProfile]               `cbor:"4,keyasint,omitempty"`
	SelectedVersion              *TEEPVersion                                  `cbor:"6,keyasint,omitempty"`
	AttestationPayload           util.BytesHexMax32                            `cbor:"7,keyasint,omitempty"`
	TCList                       util.DiagList[suit.SystemPropertyClaims]      `cbor:"8,keyasint,omitempty"`
	ExtList                      util.DiagList[TEEPExtInfo]                    `cbor:"9,keyasint,omitempty"`
	ManifestList                 util.DiagList[util.BytesHexMax32]             `cbor:"10,keyasint,omitempty"`
	Msg                          *util.DiagString                              `cbor:"11,keyasint,omitempty"`
	ErrMsg                       *util.DiagString                              `cbor:"12,keyasint,omitempty"`
	AttestationPayloadFormat     *util.DiagString                              `cbor:"13,keyasint,omitempty"`
	RequestedTCList              util.DiagList[RequestedTCInfo]                `cbor:"14,keyasint,omitempty"`
	UnneededManifestList         util.DiagList[util.BytesHexMax32]             `cbor:"15,keyasint,omitempty"`
	SUITReports                  util.DiagList[util.BytesHexMax32]             `cbor:"19,keyasint,omitempty"`
	Token                        util.BytesHexMax32                            `cbor:"20,keyasint,omitempty"`
	SupportedFreshnessMechanisms util.DiagList[FreshnessMechanism]             `cbor:"21,keyasint,omitempty"`
	ErrLang                      *util.DiagString                              `cbor:"22,keyasint,omitempty"`
	ErrCode                      *TEEPErrCode                                  `cbor:"23,keyasint,omitempty"`
}

func (o TEEPOptions) CBORDiagString(indent int) string {
	pad1 := strings.Repeat("  ", indent)
	pad2 := strings.Repeat("  ", indent+1)
	var outputStrings []string
	if o.SupportedTEEPCipherSuites != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ supported-teep-cipher-suites / 1: %v", pad2, o.SupportedTEEPCipherSuites.CBORDiagString(indent+1)))
	}
	if o.Challenge != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ challenge / 2: %x", pad2, o.Challenge.CBORDiagString(indent+1)))
	}
	if o.Versions != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ versions / 3: %v", pad2, o.Versions.CBORDiagString(indent+1)))
	}
	if o.SupportedSUITCOSEProfiles != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ supported-suit-cose-profiles / 4: %v", pad2, o.SupportedSUITCOSEProfiles.CBORDiagString(indent+1)))
	}
	if o.SelectedVersion != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ selected-version / 6: %v", pad2, *o.SelectedVersion))
	}
	if o.AttestationPayload != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ attestation-payload / 7: %x", pad2, o.AttestationPayload.CBORDiagString(indent+1)))
	}
	if o.TCList != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ tc-list / 8: %v", pad2, o.TCList.CBORDiagString(indent+1)))
	}
	if o.ExtList != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ ext-list / 9: %v", pad2, o.ExtList.CBORDiagString(indent+1)))
	}
	if o.ManifestList != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ manifest-list / 10: %v", pad2, o.ManifestList.CBORDiagString(indent+1)))
	}
	if o.Msg != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ msg / 11: %s", pad2, o.Msg.CBORDiagString(indent+1)))
	}
	if o.ErrMsg != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ err-msg / 12: %s", pad2, o.ErrMsg.CBORDiagString(indent+1)))
	}
	if o.AttestationPayloadFormat != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ attestation-payload-format / 13: %s", pad2, o.AttestationPayloadFormat.CBORDiagString(indent+1)))
	}
	if o.RequestedTCList != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ requested-tc-list / 14: %v", pad2, o.RequestedTCList.CBORDiagString(indent+1)))
	}
	if o.UnneededManifestList != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ unneeded-manifest-list / 15: %v", pad2, o.UnneededManifestList.CBORDiagString(indent+1)))
	}
	if o.SUITReports != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ suit-reports / 19: %v", pad2, o.SUITReports.CBORDiagString(indent+1)))
	}
	if o.Token != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ token / 20: %s", pad2, o.Token.CBORDiagString(indent+1)))
	}
	if o.SupportedFreshnessMechanisms != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ supported-freshness-mechanisms / 21: %v", pad2, o.SupportedFreshnessMechanisms.CBORDiagString(indent+1)))
	}
	if o.ErrLang != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ err-lang / 22: %s", pad2, o.ErrLang.CBORDiagString(indent+1)))
	}
	if o.ErrCode != nil {
		outputStrings = append(outputStrings, fmt.Sprintf("%s/ err-code / 23: %s", pad2, o.ErrCode.CBORDiagString(indent+1)))
	}
	return fmt.Sprintf("{\n%s\n%s}", strings.Join(outputStrings, ",\n"), pad1)
}

type TEEPVersion uint32

func (v TEEPVersion) CBORDiagString(indent int) string {
	return fmt.Sprintf("%d", uint32(v))
}

type TEEPExtInfo uint32

func (e TEEPExtInfo) CBORDiagString(indent int) string {
	return fmt.Sprintf("%d", uint32(e))
}

type DataItemRequested uint

const attestationMask = 0b1
const tcListMask = 0b10
const extensionsMask = 0b100
const suitReportsMask = 0b1000

func (v DataItemRequested) CBORDiagString(indent int) string {
	var items []string
	if v.AttestationRequested() {
		items = append(items, "1 / attestation /")
	}
	if v.TCListRequested() {
		items = append(items, "2 / tc-list /")
	}
	if v.ExtensionsRequested() {
		items = append(items, "3 / extensions /")
	}
	if v.SUITReportsRequested() {
		items = append(items, "4 / suit-reports /")
	}
	if len(items) == 0 {
		return "0 / none /"
	}
	return strings.Join(items, " | ")
}

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

func (e TEEPErrCode) CBORDiagString(indent int) string {
	switch e {
	case TEEPErrPermanentError:
		return "1 / ERR_PERMANENT_ERROR /"
	case TEEPErrUnsupportedExtension:
		return "2 / ERR_UNSUPPORTED_EXTENSION /"
	case TEEPErrUnsupportedFreshnessMechanisms:
		return "3 / ERR_UNSUPPORTED_FRESHNESS_MECHANISMS /"
	case TEEPErrUnsupportedMsgVersion:
		return "4 / ERR_UNSUPPORTED_MSG_VERSION /"
	case TEEPErrUnsupportedCipherSuites:
		return "5 / ERR_UNSUPPORTED_CIPHER_SUITES /"
	case TEEPErrBadCertificate:
		return "6 / ERR_BAD_CERTIFICATE /"
	case TEEPErrAttestationRequired:
		return "7 / ERR_ATTESTATION_REQUIRED /"
	case TEEPErrUnsupportedSUITReport:
		return "8 / ERR_UNSUPPORTED_SUIT_REPORT /"
	case TEEPErrCertificateExpired:
		return "9 / ERR_CERTIFICATE_EXPIRED /"
	case TEEPErrTemporaryError:
		return "10 / ERR_TEMPORARY_ERROR /"
	case TEEPErrManifestProcessingFailed:
		return "17 / ERR_MANIFEST_PROCESSING_FAILED /"
	default:
		return fmt.Sprintf("unknown(%d)", int(e))
	}
}

type TEEPCipherSuites []TEEPCipherSuite

func (c TEEPCipherSuites) CBORDiagString(indent int) string {
	var suiteStrings []string
	for _, suite := range c {
		suiteStrings = append(suiteStrings, suite.CBORDiagString(indent))
	}
	return fmt.Sprintf("[%s]", strings.Join(suiteStrings, ", "))
}

type TEEPCipherSuite struct {
	_         struct{} `cbor:",toarray"`
	Type      int      `cbor:"0,keyasint"`
	Algorithm int      `cbor:"1,keyasint"`
}

func (c TEEPCipherSuite) CBORDiagString(indent int) string {
	return fmt.Sprintf("[%d, %d]", c.Type, c.Algorithm)
}

type RequestedTCInfo struct {
	ComponentID              suit.ComponentID `cbor:"16,keyasint,omitempty"`
	TCManifestSequenceNumber *uint8           `cbor:"17,keyasint,omitempty"`
	HaveBinary               *bool            `cbor:"18,keyasint,omitempty"`
}

func (r RequestedTCInfo) CBORDiagString(indent int) string {
	var pad1 = strings.Repeat("  ", indent)
	var pad2 = strings.Repeat("  ", indent+1)
	var encodedStrings []string
	encodedStrings = append(encodedStrings, fmt.Sprintf("\n%s/ component-id / 16: %s", pad2, r.ComponentID.CBORDiagString(0)))
	if r.TCManifestSequenceNumber != nil {
		encodedStrings = append(encodedStrings, fmt.Sprintf("\n%s/ tc-manifest-sequence-number / 17: %d", pad2, *r.TCManifestSequenceNumber))
	}
	if r.HaveBinary != nil {
		encodedStrings = append(encodedStrings, fmt.Sprintf("\n%s/ have-binary / 18: %t", pad2, *r.HaveBinary))
	}
	return fmt.Sprintf("%s{\n%s\n%s}", pad1, strings.Join(encodedStrings, ", "), pad1)
}

type FreshnessMechanism uint

func (f FreshnessMechanism) CBORDiagString(indent int) string {
	return fmt.Sprintf("%d", uint(f))
}
