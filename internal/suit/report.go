/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package suit

import (
	"fmt"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

// draft-ietf-suit-report

type Report struct {
	Reference     Reference            `cbor:"99,keyasint"`
	Nonce         []byte               `cbor:"2,keyasint,omitempty"`
	ReportRecords []RecordOrClaims     `cbor:"3,keyasint"`
	Result        TrueOrDetailedResult `cbor:"4,keyasint"`
	// TODO?: support capability-report, extensions
}

type Reference struct {
	_           struct{} `cbor:",toarray"`
	ManifestUri string   `cbor:"1,keyasint"`
	Digest      Digest   `cbor:"2,keyasint"`
}

type RecordOrClaims struct {
	Record       *Record               `cbor:"0,keyasint,omitempty"`
	SystemClaims *SystemPropertyClaims `cbor:"1,keyasint,omitempty"`
}

func (rc *RecordOrClaims) UnmarshalCBOR(data []byte) error {
	var record Record
	if err := cbor.Unmarshal(data, &record); err == nil {
		rc.Record = &record
		return nil
	}

	var claims SystemPropertyClaims
	if err := cbor.Unmarshal(data, &claims); err == nil {
		rc.SystemClaims = &claims
		return nil
	}

	return fmt.Errorf("unknown type for RecordOrClaims: %T", data)
}

type TrueOrDetailedResult struct {
	True           *bool           `cbor:"0,keyasint,omitempty"`
	DetailedResult *DetailedResult `cbor:"1,keyasint,omitempty"`
}

func (tdr *TrueOrDetailedResult) UnmarshalCBOR(data []byte) error {
	var b bool
	if err := cbor.Unmarshal(data, &b); err == nil {
		if !b {
			return fmt.Errorf("simple value must be true")
		}
		tdr.True = &b
		return nil
	}

	var dr DetailedResult
	if err := cbor.Unmarshal(data, &dr); err == nil {
		tdr.DetailedResult = &dr
		return nil
	}

	return fmt.Errorf("unknown type for TrueOrDetailedResult: %T", data)
}

type Record struct {
	_                struct{}    `cbor:",toarray"`
	ManifestID       []uint      `cbor:"1,keyasint"`
	ManifestSection  int         `cbor:"2,keyasint"`
	SectionOffset    uint        `cbor:"3,keyasint"`
	ComponentIndex   uint        `cbor:"4,keyasint"`
	RecordProperties map[int]any `cbor:"5,keyasint"`
}

type DetailedResult struct {
	ResultCode   int           `cbor:"5,keyasint"`
	ResultRecord Record        `cbor:"6,keyasint"`
	ResultReason ReportReasons `cbor:"7,keyasint"`
}

type ReportReasons uint

const (
	ReportReasonOK = iota
	ReportReasonCBORParse
	ReportReasonCOSEUnsupported
	ReportReasonAlgUnsupported
	ReportReasonUnauthorised
	ReportReasonCommandUnsupported
	ReportReasonComponentUnsupported
	ReportReasonComponentUnauthorised
	ReportReasonParameterUnsupported
	ReportReasonSeveringUnsupported
	ReportReasonConditionFailed
	ReportReasonOperationFailed
	ReportReasonInvokePending
)

func (r ReportReasons) String() string {
	switch r {
	case ReportReasonOK:
		return "report-reason-ok"
	case ReportReasonCBORParse:
		return "report-reason-cbor-parse"
	case ReportReasonCOSEUnsupported:
		return "report-reason-cose-unsupported"
	case ReportReasonAlgUnsupported:
		return "report-reason-alg-unsupported"
	case ReportReasonUnauthorised:
		return "report-reason-unauthorised"
	case ReportReasonCommandUnsupported:
		return "report-reason-command-unsupported"
	case ReportReasonComponentUnsupported:
		return "report-reason-component-unsupported"
	case ReportReasonComponentUnauthorised:
		return "report-reason-component-unauthorised"
	case ReportReasonParameterUnsupported:
		return "report-reason-parameter-unsupported"
	case ReportReasonSeveringUnsupported:
		return "report-reason-severing-unsupported"
	case ReportReasonConditionFailed:
		return "report-reason-condition-failed"
	case ReportReasonOperationFailed:
		return "report-reason-operation-failed"
	case ReportReasonInvokePending:
		return "report-reason-invoke-pending"
	default:
		return "report-reason-unknown"
	}
}

type SystemPropertyClaims struct {
	SystemComponentID ComponentID `cbor:"0,keyasint,omitempty"`
	ImageDigest       []byte      `cbor:"3,keyasint,omitempty"`
	ImageSize         uint        `cbor:"14,keyasint,omitempty"`
	// NOTE: not all SUIT_Parameters are supported
}

func (s SystemPropertyClaims) CBORDiagString(indent int) string {
	pad1 := strings.Repeat("  ", indent)
	pad2 := strings.Repeat("  ", indent+1)
	var stringList []string
	if len(s.SystemComponentID) > 0 {
		stringList = append(stringList, fmt.Sprintf("%s/ system-component-id / 0: %s", pad2, s.SystemComponentID.CBORDiagString(indent+1)))
	}
	if len(s.ImageDigest) > 0 {
		var digest Digest
		if err := cbor.Unmarshal(s.ImageDigest, &digest); err == nil {
			stringList = append(stringList, fmt.Sprintf("%s/ image-digest / 3: << %s >>", pad2, digest.CBORDiagString(indent+1)))
		}
	}
	if s.ImageSize != 0 {
		stringList = append(stringList, fmt.Sprintf("%s/ image-size / 14: %d", pad2, s.ImageSize))
	}
	return fmt.Sprintf("{\n%s\n%s}", strings.Join(stringList, ",\n"), pad1)
}
