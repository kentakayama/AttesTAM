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
		NULL,
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
*/
import "C"

import (
	"fmt"
	"log"
	"time"

	"github.com/kentakayama/AttesTAM/internal/config"
)

type IntelQVLVerifier struct {
	logger *log.Logger
}

func NewIntelQVLVerifier(cfg config.RAConfig) (IRAVerifier, error) {
	return &IntelQVLVerifier{logger: cfg.Logger}, nil
}

func (v *IntelQVLVerifier) Process(payload []byte) (*ProcessedAttestation, error) {
	decoded, err := decodeIntelQVLAttestationPayload(payload)
	if err != nil {
		return nil, err
	}
	if len(decoded.RawReportData) == 0 {
		return nil, fmt.Errorf("intel-qvl attestation payload missing raw-report-data")
	}

	expectedReportData, err := buildIntelQVLExpectedReportData(decoded.RawReportData)
	if err != nil {
		return nil, err
	}
	if err := verifyQuoteReportData(decoded.Quote, expectedReportData); err != nil {
		return nil, err
	}

	cQuote := C.CBytes(decoded.Quote)
	defer C.free(cQuote)

	var nativeResult C.attestam_qvl_result_t
	status := C.attestam_qvl_verify_quote(
		(*C.uint8_t)(cQuote),
		C.uint32_t(len(decoded.Quote)),
		C.time_t(time.Now().Unix()),
		&nativeResult,
	)
	if status != C.TEE_SUCCESS {
		return nil, fmt.Errorf("tee_verify_quote failed: 0x%04x", uint32(nativeResult.dcap_status))
	}

	ok := nativeResult.verification_result == C.TEE_QV_RESULT_OK && nativeResult.collateral_expiration_status == 0
	result := &ProcessedAttestation{
		Backend:    VerifierBackendIntelQVL,
		EarStatus:  resultToEarStatus(ok),
		SendUpdate: false,
	}
	if err := validateIntelQVLClaims(decoded.RawReportData, result); err != nil {
		return nil, err
	}
	if v.logger != nil {
		v.logger.Printf("Intel QVL verification result: dcap_status=0x%04x verification_result=0x%x collateral_expiration_status=%d supplemental_data_size=%d",
			uint32(nativeResult.dcap_status),
			uint32(nativeResult.verification_result),
			uint32(nativeResult.collateral_expiration_status),
			uint32(nativeResult.supplemental_data_size),
		)
	}

	return result, nil
}
