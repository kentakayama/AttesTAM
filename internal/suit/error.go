/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package suit

import "errors"

var (
	ErrFatal                             = errors.New("fatal error occured")
	ErrNotSupported                      = errors.New("not supported")
	ErrInvalidType                       = errors.New("invalid type")
	ErrInvalidValue                      = errors.New("invalid value")
	ErrSUITManifestInvalidFormat         = errors.New("invalid SUIT manifest")
	ErrSUITManifestNotAuthenticated      = errors.New("SUIT manifest not authenticated")
	ErrSUITManifestMissingKID            = errors.New("SUIT authentication block does not contain a kid")
	ErrSUITManifestSmallerSequenceNumber = errors.New("exising SUIT manifest has bigger sequence-number")
	ErrSUITManifestSigningKeyMismatch    = errors.New("existing SUIT manifest is signed by another entity")
)
