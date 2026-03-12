/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package model

import (
	"time"

	"github.com/kentakayama/AttesTAM/internal/util"
)

// SuitManifest represents a SUIT manifest stored in DB.
type SuitManifest struct {
	ID                 int64
	Manifest           []byte
	Digest             []byte // encoded SUIT_Digest, i.e. [-16, h'deadbeef...']
	SigningKeyID       int64
	TrustedComponentID []byte
	SequenceNumber     uint64
	CreatedAt          time.Time
}

// SuitManifestOverview represents the digest of SUIT manifest for the TC Developers, Device Admins, ...
type SuitManifestOverview struct {
	_                  struct{}           `cbor:",toarray"`
	TrustedComponentID util.BytesHexMax32 `cbor:"0,keyasint"`
	SequenceNumber     uint64             `cbor:"1,keyasint"`
}
