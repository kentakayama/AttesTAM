/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package tam

import "errors"

var (
	ErrFatal                      = errors.New("fatal error occured")
	ErrNotTEEPMessage             = errors.New("not a TEEP Message")
	ErrNotSupported               = errors.New("not supported")
	ErrInvalidType                = errors.New("invalid type")
	ErrInvalidValue               = errors.New("invalid value")
	ErrKidIsMissing               = errors.New("kid is missing")
	ErrNotAuthenticated           = errors.New("not authenticated")
	ErrAttestationFailed          = errors.New("attestataion failed")
	ErrNotAResponse               = errors.New("corresponding sent message not found")
	ErrAttestationPayloadNotFound = errors.New("attestation payload not found")
)
