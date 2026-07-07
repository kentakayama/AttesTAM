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

static quote3_error_t attestam_qvl_verify_quote_with_collateral_fields(
	const uint8_t *quote,
	uint32_t quote_size,
	uint16_t collateral_major_version,
	uint16_t collateral_minor_version,
	uint32_t collateral_tee_type,
	const char *pck_crl_issuer_chain,
	uint32_t pck_crl_issuer_chain_size,
	const char *root_ca_crl,
	uint32_t root_ca_crl_size,
	const char *pck_crl,
	uint32_t pck_crl_size,
	const char *tcb_info_issuer_chain,
	uint32_t tcb_info_issuer_chain_size,
	const char *tcb_info,
	uint32_t tcb_info_size,
	const char *qe_identity_issuer_chain,
	uint32_t qe_identity_issuer_chain_size,
	const char *qe_identity,
	uint32_t qe_identity_size,
	const time_t current_time,
	attestam_qvl_result_t *result
) {
	sgx_ql_qve_collateral_t collateral;
	memset(&collateral, 0, sizeof(collateral));
	collateral.major_version = collateral_major_version;
	collateral.minor_version = collateral_minor_version;
	collateral.tee_type = collateral_tee_type;
	collateral.pck_crl_issuer_chain = (char *)pck_crl_issuer_chain;
	collateral.pck_crl_issuer_chain_size = pck_crl_issuer_chain_size;
	collateral.root_ca_crl = (char *)root_ca_crl;
	collateral.root_ca_crl_size = root_ca_crl_size;
	collateral.pck_crl = (char *)pck_crl;
	collateral.pck_crl_size = pck_crl_size;
	collateral.tcb_info_issuer_chain = (char *)tcb_info_issuer_chain;
	collateral.tcb_info_issuer_chain_size = tcb_info_issuer_chain_size;
	collateral.tcb_info = (char *)tcb_info;
	collateral.tcb_info_size = tcb_info_size;
	collateral.qe_identity_issuer_chain = (char *)qe_identity_issuer_chain;
	collateral.qe_identity_issuer_chain_size = qe_identity_issuer_chain_size;
	collateral.qe_identity = (char *)qe_identity;
	collateral.qe_identity_size = qe_identity_size;

	return attestam_qvl_verify_quote(
		quote,
		quote_size,
		(const uint8_t *)&collateral,
		current_time,
		result
	);
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

type intelQVLNativeResult struct {
	collateralExpirationStatus uint32
	verificationResult         uint32
	supplementalDataSize       uint32
	dcapStatus                 uint32
}

var (
	intelQVLQuoteVerifier         = verifyIntelQVLQuote
	intelQVLQuoteCollateralGetter = getIntelQVLQuoteCollateral
)

type IntelQVLVerifier struct {
	logger          *log.Logger
	collateralCache *intelQVLCollateralCache
	pcsClient       *intelQVLPCSClient
}

func NewIntelQVLVerifier(cfg config.RAConfig) (IRAVerifier, error) {
	cache, err := newIntelQVLCollateralCache(cfg.IntelCollateralCacheDir, cfg.Logger)
	if err != nil {
		return nil, err
	}
	pcsClient, err := newIntelQVLPCSClient(cfg)
	if err != nil {
		return nil, err
	}
	return &IntelQVLVerifier{
		logger:          cfg.Logger,
		collateralCache: cache,
		pcsClient:       pcsClient,
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
			v.logger.Printf("Intel QVL collateral cache hit: key=%s", cacheKey)
		} else {
			v.logger.Printf("Intel QVL collateral cache miss: key=%s", cacheKey)
		}
	}

	status, nativeResult := intelQVLQuoteVerifier(cQuote, len(payload), collateral)
	if status != uint32(C.TEE_SUCCESS) {
		if v.logger != nil {
			v.logNativeResult(&nativeResult, intelQVLClassificationFailure, resultToEarStatus(false))
		}
		return nil, fmt.Errorf("tee_verify_quote failed: 0x%04x", nativeResult.dcapStatus)
	}

	classification := classifyIntelQVLResult(
		nativeResult.verificationResult,
		nativeResult.collateralExpirationStatus,
	)
	ok := isIntelQVLResultAffirmingValue(nativeResult.verificationResult, nativeResult.collateralExpirationStatus)
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

func (v *IntelQVLVerifier) quoteCollateral(quote []byte, cQuote unsafe.Pointer) (*intelQVLQuoteCollateral, bool, string, error) {
	if v.collateralCache != nil {
		collateral, cacheHit, cacheKey, err := v.collateralCache.Get(quote)
		if err != nil || cacheHit {
			return collateral, cacheHit, cacheKey, err
		}
		collateral, err = intelQVLQuoteCollateralGetter(v.pcsClient, quote, cQuote, len(quote))
		if err != nil {
			return nil, false, cacheKey, err
		}
		if collateral != nil {
			if key, err := v.collateralCache.Put(quote, collateral); err != nil {
				if v.logger != nil {
					v.logger.Printf("failed to store Intel QVL collateral cache: %v", err)
				}
			} else if v.logger != nil {
				v.logger.Printf("stored Intel QVL collateral cache: key=%s", key)
			}
		}
		return collateral, false, cacheKey, nil
	}
	collateral, err := intelQVLQuoteCollateralGetter(v.pcsClient, quote, cQuote, len(quote))
	if err != nil {
		return nil, false, "", err
	}
	return collateral, false, "", nil
}

func verifyIntelQVLQuote(cQuote unsafe.Pointer, quoteSize int, collateral *intelQVLQuoteCollateral) (uint32, intelQVLNativeResult) {
	var nativeResult C.attestam_qvl_result_t
	if collateral == nil {
		return uint32(C.TEE_ERROR_INVALID_PARAMETER), intelQVLNativeResult{
			dcapStatus: uint32(C.TEE_ERROR_INVALID_PARAMETER),
		}
	}
	var (
		cPCKCRLIssuerChain     unsafe.Pointer
		cRootCACRL             unsafe.Pointer
		cPCKCRL                unsafe.Pointer
		cTCBInfoIssuerChain    unsafe.Pointer
		cTCBInfo               unsafe.Pointer
		cQEIdentityIssuerChain unsafe.Pointer
		cQEIdentity            unsafe.Pointer
	)
	pckCRLIssuerChain := bytesWithTerminatingNUL(collateral.PCKCRLIssuerChain)
	rootCACRL := intelQVLCRLBytesForNative(collateral.MajorVersion, collateral.MinorVersion, collateral.RootCACRL)
	pckCRL := intelQVLCRLBytesForNative(collateral.MajorVersion, collateral.MinorVersion, collateral.PCKCRL)
	tcbInfoIssuerChain := bytesWithTerminatingNUL(collateral.TCBInfoIssuerChain)
	tcbInfo := bytesWithTerminatingNUL(collateral.TCBInfo)
	qeIdentityIssuerChain := bytesWithTerminatingNUL(collateral.QEIdentityIssuerChain)
	qeIdentity := bytesWithTerminatingNUL(collateral.QEIdentity)

	cPCKCRLIssuerChain = cBytesOrNil(pckCRLIssuerChain)
	defer C.free(cPCKCRLIssuerChain)
	cRootCACRL = cBytesOrNil(rootCACRL)
	defer C.free(cRootCACRL)
	cPCKCRL = cBytesOrNil(pckCRL)
	defer C.free(cPCKCRL)
	cTCBInfoIssuerChain = cBytesOrNil(tcbInfoIssuerChain)
	defer C.free(cTCBInfoIssuerChain)
	cTCBInfo = cBytesOrNil(tcbInfo)
	defer C.free(cTCBInfo)
	cQEIdentityIssuerChain = cBytesOrNil(qeIdentityIssuerChain)
	defer C.free(cQEIdentityIssuerChain)
	cQEIdentity = cBytesOrNil(qeIdentity)
	defer C.free(cQEIdentity)

	status := C.attestam_qvl_verify_quote_with_collateral_fields(
		(*C.uint8_t)(cQuote),
		C.uint32_t(quoteSize),
		C.uint16_t(collateral.MajorVersion),
		C.uint16_t(collateral.MinorVersion),
		C.uint32_t(collateral.TEEType),
		(*C.char)(cPCKCRLIssuerChain),
		C.uint32_t(len(pckCRLIssuerChain)),
		(*C.char)(cRootCACRL),
		C.uint32_t(len(rootCACRL)),
		(*C.char)(cPCKCRL),
		C.uint32_t(len(pckCRL)),
		(*C.char)(cTCBInfoIssuerChain),
		C.uint32_t(len(tcbInfoIssuerChain)),
		(*C.char)(cTCBInfo),
		C.uint32_t(len(tcbInfo)),
		(*C.char)(cQEIdentityIssuerChain),
		C.uint32_t(len(qeIdentityIssuerChain)),
		(*C.char)(cQEIdentity),
		C.uint32_t(len(qeIdentity)),
		C.time_t(time.Now().Unix()),
		&nativeResult,
	)
	return uint32(status), intelQVLNativeResult{
		collateralExpirationStatus: uint32(nativeResult.collateral_expiration_status),
		verificationResult:         uint32(nativeResult.verification_result),
		supplementalDataSize:       uint32(nativeResult.supplemental_data_size),
		dcapStatus:                 uint32(nativeResult.dcap_status),
	}
}

func getIntelQVLQuoteCollateral(pcsClient *intelQVLPCSClient, quote []byte, cQuote unsafe.Pointer, quoteSize int) (*intelQVLQuoteCollateral, error) {
	if pcsClient == nil {
		return nil, fmt.Errorf("Intel PCS client is not configured")
	}
	return pcsClient.Fetch(quote)
}

func (v *IntelQVLVerifier) logNativeResult(nativeResult *intelQVLNativeResult, classification string, earStatus string) {
	v.logger.Printf("Intel QVL verification result: classification=%s ear_status=%s dcap_status=0x%04x verification_result_name=%s verification_result=0x%x collateral_expiration_status=%d supplemental_data_size=%d",
		classification,
		earStatus,
		nativeResult.dcapStatus,
		intelQVLVerificationResultName(nativeResult.verificationResult),
		nativeResult.verificationResult,
		nativeResult.collateralExpirationStatus,
		nativeResult.supplementalDataSize,
	)
}

func isIntelQVLResultAffirming(verificationResult C.sgx_ql_qv_result_t, collateralExpirationStatus C.uint32_t) bool {
	return isIntelQVLResultAffirmingValue(uint32(verificationResult), uint32(collateralExpirationStatus))
}

func cBytesOrNil(data []byte) unsafe.Pointer {
	if len(data) == 0 {
		return nil
	}
	return C.CBytes(data)
}

func bytesWithTerminatingNUL(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	if data[len(data)-1] == 0 {
		return data
	}
	out := make([]byte, len(data)+1)
	copy(out, data)
	return out
}

func intelQVLCRLBytesForNative(majorVersion, minorVersion uint16, data []byte) []byte {
	if majorVersion == 3 && minorVersion == 1 {
		return data
	}
	return bytesWithTerminatingNUL(data)
}
