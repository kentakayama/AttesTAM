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
#include <string.h>
#include <time.h>
#include <sgx_dcap_quoteverify.h>

typedef struct attestam_qvl_result_t {
	uint32_t collateral_expiration_status;
	sgx_ql_qv_result_t verification_result;
	uint32_t supplemental_data_size;
	quote3_error_t dcap_status;
} attestam_qvl_result_t;

static quote3_error_t attestam_qvl_verify_quote(
	const uint8_t *quote,
	uint32_t quote_size,
	const uint8_t *quote_collateral,
	const time_t current_time,
	attestam_qvl_result_t *result
) {
	tee_supp_data_descriptor_t supp_data;
	memset(&supp_data, 0, sizeof(supp_data));

	uint32_t latest_version = 0;
	quote3_error_t status = tee_get_supplemental_data_version_and_size(
		quote,
		quote_size,
		&latest_version,
		&supp_data.data_size
	);
	if (status == TEE_SUCCESS && supp_data.data_size > 0) {
		supp_data.p_data = (uint8_t*)calloc(1, supp_data.data_size);
		if (supp_data.p_data == NULL) {
			supp_data.data_size = 0;
		}
	} else {
		supp_data.data_size = 0;
	}

	result->collateral_expiration_status = 1;
	result->verification_result = TEE_QV_RESULT_UNSPECIFIED;
	result->supplemental_data_size = supp_data.data_size;

	status = tee_verify_quote(
		quote,
		quote_size,
		quote_collateral,
		current_time,
		&result->collateral_expiration_status,
		&result->verification_result,
		NULL,
		&supp_data
	);
	result->dcap_status = status;

	if (supp_data.p_data != NULL) {
		free(supp_data.p_data);
	}

	return status;
}

static quote3_error_t attestam_qvl_get_collateral(
	const uint8_t *quote,
	uint32_t quote_size,
	uint8_t **collateral,
	uint32_t *collateral_size
) {
	return tee_qv_get_collateral(quote, quote_size, collateral, collateral_size);
}

static quote3_error_t attestam_qvl_free_collateral(uint8_t *collateral) {
	return tee_qv_free_collateral(collateral);
}

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
	"log"
	"time"
	"unsafe"

	"github.com/kentakayama/AttesTAM/internal/config"
)

type IntelQVLVerifier struct {
	logger          *log.Logger
	collateralCache *intelQVLCollateralCache
}

func NewIntelQVLVerifier(cfg config.RAConfig) (IRAVerifier, error) {
	cache, err := newIntelQVLCollateralCache(cfg.IntelQVLCollateralCacheDir, cfg.Logger)
	if err != nil {
		return nil, err
	}
	return &IntelQVLVerifier{
		logger:          cfg.Logger,
		collateralCache: cache,
	}, nil
}

func (v *IntelQVLVerifier) Process(payload []byte) (*ProcessedAttestation, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("intel-qvl quote is empty")
	}

	cQuote := C.CBytes(payload)
	defer C.free(cQuote)

	collateral, cacheHit, cacheKey, err := v.quoteCollateral(payload, cQuote)
	if err != nil {
		return nil, err
	}
	if v.logger != nil && v.collateralCache != nil {
		if cacheHit {
			v.logger.Printf("Intel QVL collateral cache hit: key=%s size=%d", cacheKey, len(collateral))
		} else {
			v.logger.Printf("Intel QVL collateral cache miss: key=%s", cacheKey)
		}
	}

	status, nativeResult := verifyIntelQVLQuote(cQuote, len(payload), collateral)
	if status != C.TEE_SUCCESS {
		if v.logger != nil {
			v.logNativeResult(&nativeResult, intelQVLClassificationFailure, resultToEarStatus(false))
		}
		return nil, fmt.Errorf("tee_verify_quote failed: 0x%04x", uint32(nativeResult.dcap_status))
	}
	if v.collateralCache != nil && !cacheHit && len(collateral) > 0 {
		if key, err := v.collateralCache.Put(payload, collateral); err != nil {
			if v.logger != nil {
				v.logger.Printf("failed to store Intel QVL collateral cache: %v", err)
			}
		} else if v.logger != nil {
			v.logger.Printf("stored Intel QVL collateral cache: key=%s size=%d", key, len(collateral))
		}
	}

	classification := classifyIntelQVLResult(
		uint32(nativeResult.verification_result),
		uint32(nativeResult.collateral_expiration_status),
	)
	ok := isIntelQVLResultAffirming(nativeResult.verification_result, nativeResult.collateral_expiration_status)
	result := &ProcessedAttestation{
		Backend:    VerifierBackendIntelQVL,
		EarStatus:  resultToEarStatus(ok),
		SendUpdate: false,
	}
	if v.logger != nil {
		v.logNativeResult(&nativeResult, classification, result.EarStatus)
	}

	return result, nil
}

func (v *IntelQVLVerifier) quoteCollateral(quote []byte, cQuote unsafe.Pointer) ([]byte, bool, string, error) {
	if v.collateralCache == nil {
		return nil, false, "", nil
	}
	collateral, cacheHit, cacheKey, err := v.collateralCache.Get(quote)
	if err != nil || cacheHit {
		return collateral, cacheHit, cacheKey, err
	}
	collateral, err = getIntelQVLQuoteCollateral(cQuote, len(quote))
	if err != nil {
		return nil, false, cacheKey, err
	}
	return collateral, false, cacheKey, nil
}

func verifyIntelQVLQuote(cQuote unsafe.Pointer, quoteSize int, collateral []byte) (C.quote3_error_t, C.attestam_qvl_result_t) {
	var nativeResult C.attestam_qvl_result_t
	var cCollateral unsafe.Pointer
	if len(collateral) > 0 {
		cCollateral = C.CBytes(collateral)
		defer C.free(cCollateral)
	}
	status := C.attestam_qvl_verify_quote(
		(*C.uint8_t)(cQuote),
		C.uint32_t(quoteSize),
		(*C.uint8_t)(cCollateral),
		C.time_t(time.Now().Unix()),
		&nativeResult,
	)
	return status, nativeResult
}

func getIntelQVLQuoteCollateral(cQuote unsafe.Pointer, quoteSize int) ([]byte, error) {
	var cCollateral *C.uint8_t
	var cCollateralSize C.uint32_t
	status := C.attestam_qvl_get_collateral(
		(*C.uint8_t)(cQuote),
		C.uint32_t(quoteSize),
		&cCollateral,
		&cCollateralSize,
	)
	if status != C.TEE_SUCCESS {
		return nil, fmt.Errorf("tee_qv_get_collateral failed: 0x%04x", uint32(status))
	}
	if cCollateral == nil || cCollateralSize == 0 {
		return nil, fmt.Errorf("tee_qv_get_collateral returned empty collateral")
	}
	defer C.attestam_qvl_free_collateral(cCollateral)
	return C.GoBytes(unsafe.Pointer(cCollateral), C.int(cCollateralSize)), nil
}

func extractIntelQVLFMSPC(quote []byte) ([]byte, error) {
	if len(quote) == 0 {
		return nil, fmt.Errorf("Intel QVL quote is empty")
	}
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

func (v *IntelQVLVerifier) logNativeResult(nativeResult *C.attestam_qvl_result_t, classification string, earStatus string) {
	v.logger.Printf("Intel QVL verification result: classification=%s ear_status=%s dcap_status=0x%04x verification_result_name=%s verification_result=0x%x collateral_expiration_status=%d supplemental_data_size=%d",
		classification,
		earStatus,
		uint32(nativeResult.dcap_status),
		intelQVLVerificationResultName(uint32(nativeResult.verification_result)),
		uint32(nativeResult.verification_result),
		uint32(nativeResult.collateral_expiration_status),
		uint32(nativeResult.supplemental_data_size),
	)
}

func isIntelQVLResultAffirming(verificationResult C.sgx_ql_qv_result_t, collateralExpirationStatus C.uint32_t) bool {
	return isIntelQVLResultAffirmingValue(uint32(verificationResult), uint32(collateralExpirationStatus))
}
