/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package suit

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fxamacker/cbor/v2"
	"github.com/veraison/go-cose"
)

const (
	EnvelopeTag         = 107
	envelopeTagBytesLen = 2
	ManifestTag         = 1070
	manifestTagBytesLen = 3
)

var (
	envelopeTagBytes = []byte{0xD8, 0x6B}
	manifestTagBytes = []byte{0xD9, 0x04, 0x2E}
)

type Nested[T any] struct {
	Value T
}

func (n *Nested[T]) UnmarshalCBOR(data []byte) error {
	// data is bstr wrapped something
	var raw []byte
	if err := cbor.Unmarshal(data, &raw); err != nil {
		return err
	}
	// raw is the content
	return cbor.Unmarshal(raw, &n.Value)
}

// Envelope is a TAM-focused parser/validator for draft-ietf-suit-manifest payloads.
// The implementation is intentionally limited to what TAM needs for manifest
// registration and verification, and should not be treated as a full SUIT library.

type Envelope struct {
	Tagged                bool
	AuthenticationWrapper Nested[AuthenticationWrapper]
	ManifestBstr          cbor.RawMessage
}

func (e *Envelope) UnmarshalCBOR(data []byte) error {
	e.Tagged, data = e.SkipTag(data)
	var t map[any]cbor.RawMessage
	err := cbor.Unmarshal(data, &t)
	if err != nil {
		return ErrSUITManifestInvalidFormat
	}

	authenticationWrapperRaw := t[uint64(2)]
	if authenticationWrapperRaw == nil {
		return ErrSUITManifestInvalidFormat
	}
	err = cbor.Unmarshal(authenticationWrapperRaw, &e.AuthenticationWrapper)
	if err != nil {
		return err
	}

	e.ManifestBstr = t[uint64(3)]
	if e.ManifestBstr == nil {
		return ErrSUITManifestInvalidFormat
	}

	return nil
}

func (e *Envelope) SkipTag(data []byte) (bool, []byte) {
	// rough tag check
	if bytes.Equal(data[:envelopeTagBytesLen], envelopeTagBytes) {
		// tag exists, skip the data
		return true, data[envelopeTagBytesLen:]
	} else {
		return false, data
	}
}

func (e *Envelope) Verify(key *cose.Key) error {
	// Verify a SUIT Manifest based on the following sections:
	// draft-ietf-suit-manifest-34#section-6.2
	// draft-ietf-suit-manifest-34#section-8.3

	// step 1: compare the digest with hash(bstr-wrapped SUIT_Manifest)
	var digest Digest
	if err := cbor.Unmarshal(e.AuthenticationWrapper.Value.DigestBstr, &digest); err != nil {
		return ErrSUITManifestNotAuthenticated
	}
	switch digest.DigestAlg {
	case cose.AlgorithmSHA256: // SHA-256
		if !crypto.SHA256.Available() {
			return ErrSUITManifestNotAuthenticated
		}
		h := crypto.SHA256.New()
		h.Write(e.ManifestBstr)
		s := h.Sum(nil)
		if !bytes.Equal(digest.DigestBytes, s) {
			return ErrSUITManifestNotAuthenticated
		}
	default:
		return ErrSUITManifestNotAuthenticated
	}

	// step 2: verify the authentication block with specified key
	publicKey, err := key.PublicKey()
	if err != nil {
		return ErrFatal
	}
	verifier, err := cose.NewVerifier(key.Algorithm, publicKey)
	if err != nil {
		return ErrFatal
	}

	for i := 0; i < len(e.AuthenticationWrapper.Value.AuthenticationBlocks); i++ {
		var sign1 Nested[cose.Sign1Message]
		err = cbor.Unmarshal(e.AuthenticationWrapper.Value.AuthenticationBlocks[i].authenticationBlockBstr, &sign1)
		if err != nil {
			// TODO: only COSE_Sign1 is supported
			return ErrSUITManifestNotAuthenticated
		}
		if sign1.Value.Payload != nil {
			return ErrSUITManifestInvalidFormat
		}
		sign1.Value.Payload = e.AuthenticationWrapper.Value.DigestBstr
		err = sign1.Value.Verify(nil, verifier)
		if err == nil {
			// authenticated
			return nil
		}
	}
	return ErrSUITManifestNotAuthenticated
}

type AuthenticationWrapper struct {
	// the bstr-wrapped SUIT_Digest
	DigestBstr []byte
	// NOTE: accepts manifests with only one authentication block
	//
	// To avoid deciding these options below, we are currently limits the number of authentication-block to ONE.
	// Possible option 1) it is REQUIRED the TAM to verify ALL of signatures in a manifest
	// Possible option 2) it is REQUIRED the TAM to verify at least ONE of signature in a manifest
	AuthenticationBlocks [1]AuthenticationBlock
}

type AuthenticationBlock struct {
	KID                     []byte
	authenticationBlockBstr cbor.RawMessage
}

func (a *AuthenticationWrapper) UnmarshalCBOR(data []byte) error {
	var suitAuthenticationElements []cbor.RawMessage
	if err := cbor.Unmarshal(data, &suitAuthenticationElements); err != nil {
		return ErrSUITManifestInvalidFormat
	}
	// requires bstr .cbor SUIT_Digest and exactly ONE bstr .cbor SUIT_Authenticatin_Block
	if len(suitAuthenticationElements) != 2 {
		return ErrSUITManifestInvalidFormat
	}
	if err := cbor.Unmarshal(suitAuthenticationElements[0], &a.DigestBstr); err != nil {
		return ErrSUITManifestInvalidFormat
	}
	a.AuthenticationBlocks[0].authenticationBlockBstr = suitAuthenticationElements[1]

	// extract kid from unprotected header
	var sign1 Nested[cose.Sign1Message]
	if err := cbor.Unmarshal(suitAuthenticationElements[1], &sign1); err != nil {
		return ErrSUITManifestInvalidFormat
	}
	if a.AuthenticationBlocks[0].KID = extractKID(sign1.Value); a.AuthenticationBlocks[0].KID == nil {
		return ErrSUITManifestMissingKID
	}

	return nil
}

func extractKID(sign1 cose.Sign1Message) []byte {
	if p4, ok := sign1.Headers.Protected[int64(4)]; ok {
		if kid, ok := p4.([]byte); ok {
			return kid
		}
		return nil
	}
	if u4, ok := sign1.Headers.Unprotected[int64(4)]; ok {
		if kid, ok := u4.([]byte); ok {
			return kid
		}
		return nil
	}
	return nil
}

type Manifest struct {
	ManifestVersion        uint64         `cbor:"1,keyasint"`
	ManifestSequenceNumber uint64         `cbor:"2,keyasint"`
	Common                 Nested[Common] `cbor:"3,keyasint"`
}

type Common struct {
	Components     []ComponentID `cbor:"2,keyasint,omitempty"`
	SharedSequence []byte        `cbor:"4,keyasint,omitempty"` // no need to extract SUIT_Shared_Sequence
}

type ComponentID []ComponentIDBytes

func (c *ComponentID) CBORDiagString(indent int) string {
	var s []string
	for i := range *c {
		s = append(s, (*c)[i].CBORDiagString(indent))
	}
	return fmt.Sprintf("[%s]", strings.Join(s, ", "))
}

type ComponentIDBytes []byte

// String returns a human-readable representation of the ComponentIDBytes with CBOR Diagnostic Notation.
func (c ComponentIDBytes) CBORDiagString(indent int) string {
	// try to represent with UTF-8 string => 'foo-bar'
	utf8EncodedString := string(c)
	if utf8.ValidString(utf8EncodedString) && isPrintableUTF8(c) {
		return fmt.Sprintf("'%s'", utf8EncodedString)
	}
	// otherwise, represent with hex string => h'1234abcd'
	return fmt.Sprintf("h'%s'", hex.EncodeToString(c))
}

func isPrintableUTF8(b []byte) bool {
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			return false // invalid rune
		}
		if !unicode.IsPrint(r) {
			return false // non-printable rune
		}
		b = b[size:]
	}
	return true
}

type Digest struct {
	_           struct{}       `cbor:",toarray"`
	DigestAlg   cose.Algorithm `cbor:"0,keyasint"` // SHA-256 (-16), etc.
	DigestBytes []byte         `cbor:"1,keyasint"`
}

func (d Digest) CBORDiagString(indent int) string {
	return fmt.Sprintf("[%d, h'%s']", d.DigestAlg, strings.ToUpper(hex.EncodeToString(d.DigestBytes)))
}

// draft-ietf-suit-mti
type COSEProfile struct {
	_              struct{}       `cbor:",toarray"`
	DigestAlg      cose.Algorithm `cbor:"0,keyasint"` // SHA-256 (-16), etc.
	AuthAlg        cose.Algorithm `cbor:"1,keyasint"` // HMAC w/ SHA-256 (5), ESP256 (-9), etc.
	KeyExchangeAlg cose.Algorithm `cbor:"2,keyasint"` // ECDH-ES+A128KW (-29), etc.
	EncryptionAlg  cose.Algorithm `cbor:"3,keyasint"` // AES-CTR (-65534), AES-GCM (1), ChaCha/Poly (24), etc.
}

func (p COSEProfile) CBORDiagString(indent int) string {
	return fmt.Sprintf("[%d, %d, %d, %d]", p.DigestAlg, p.AuthAlg, p.KeyExchangeAlg, p.EncryptionAlg)
}
