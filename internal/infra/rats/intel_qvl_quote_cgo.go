//go:build intel_qvl && cgo

/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

/*
#cgo LDFLAGS: -lsgx_dcap_quoteverify
#include <stdlib.h>
#include <sgx_dcap_quoteverify.h>

static quote3_error_t attestam_qvl_get_fmspc(
	const uint8_t *quote,
	uint32_t quote_size,
	uint8_t *fmspc,
	uint32_t fmspc_size
) {
	return tee_get_fmspc_from_quote(quote, quote_size, fmspc, fmspc_size);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func extractIntelQVLFMSPCNative(quote []byte) ([]byte, error) {
	cQuote := C.CBytes(quote)
	defer C.free(cQuote)

	fmspc := make([]byte, intelQVLFMSPCSize)
	status := C.attestam_qvl_get_fmspc(
		(*C.uint8_t)(cQuote),
		C.uint32_t(len(quote)),
		(*C.uint8_t)(unsafe.Pointer(&fmspc[0])),
		C.uint32_t(len(fmspc)),
	)
	if status != C.TEE_SUCCESS {
		return nil, fmt.Errorf("tee_get_fmspc_from_quote failed: 0x%04x", uint32(status))
	}
	return fmspc, nil
}
