/*
 * Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package resources

import (
	_ "embed"
)

var (
	//go:embed tam_priv.cbor
	TAMCoseKeyBytes []byte
)
