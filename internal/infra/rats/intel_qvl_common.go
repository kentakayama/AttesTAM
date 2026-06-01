/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"fmt"
)

const intelQVLNonAffirming = "non-affirming"

const (
	intelQVLClassificationOK             = "ok"
	intelQVLClassificationOKWithWarnings = "ok-with-warnings"
	intelQVLClassificationFailure        = "failure"
)

const (
	intelQVLResultOK                            uint32 = 0x0000
	intelQVLResultConfigNeeded                  uint32 = 0x0000a001
	intelQVLResultOutOfDate                     uint32 = 0x0000a002
	intelQVLResultOutOfDateConfigNeeded         uint32 = 0x0000a003
	intelQVLResultInvalidSignature              uint32 = 0x0000a004
	intelQVLResultRevoked                       uint32 = 0x0000a005
	intelQVLResultUnspecified                   uint32 = 0x0000a006
	intelQVLResultSWHardeningNeeded             uint32 = 0x0000a007
	intelQVLResultConfigAndSWHardeningNeeded    uint32 = 0x0000a008
	intelQVLResultTDRelaunchAdvised             uint32 = 0x0000a009
	intelQVLResultTDRelaunchAdvisedConfigNeeded uint32 = 0x0000a00a
)

func resultToEarStatus(ok bool) string {
	if ok {
		return "affirming"
	}
	return intelQVLNonAffirming
}

func classifyIntelQVLResult(verificationResult uint32, collateralExpirationStatus uint32) string {
	if collateralExpirationStatus != 0 {
		return intelQVLClassificationFailure
	}

	switch verificationResult {
	case intelQVLResultOK:
		return intelQVLClassificationOK
	case intelQVLResultConfigNeeded,
		intelQVLResultOutOfDate,
		intelQVLResultOutOfDateConfigNeeded,
		intelQVLResultSWHardeningNeeded,
		intelQVLResultConfigAndSWHardeningNeeded,
		intelQVLResultTDRelaunchAdvised,
		intelQVLResultTDRelaunchAdvisedConfigNeeded:
		return intelQVLClassificationOKWithWarnings
	default:
		return intelQVLClassificationFailure
	}
}

func isIntelQVLResultAffirmingValue(verificationResult uint32, collateralExpirationStatus uint32) bool {
	classification := classifyIntelQVLResult(verificationResult, collateralExpirationStatus)
	return classification == intelQVLClassificationOK || classification == intelQVLClassificationOKWithWarnings
}

func intelQVLVerificationResultName(verificationResult uint32) string {
	switch verificationResult {
	case intelQVLResultOK:
		return "TEE_QV_RESULT_OK"
	case intelQVLResultConfigNeeded:
		return "TEE_QV_RESULT_CONFIG_NEEDED"
	case intelQVLResultOutOfDate:
		return "TEE_QV_RESULT_OUT_OF_DATE"
	case intelQVLResultOutOfDateConfigNeeded:
		return "TEE_QV_RESULT_OUT_OF_DATE_CONFIG_NEEDED"
	case intelQVLResultInvalidSignature:
		return "TEE_QV_RESULT_INVALID_SIGNATURE"
	case intelQVLResultRevoked:
		return "TEE_QV_RESULT_REVOKED"
	case intelQVLResultUnspecified:
		return "TEE_QV_RESULT_UNSPECIFIED"
	case intelQVLResultSWHardeningNeeded:
		return "TEE_QV_RESULT_SW_HARDENING_NEEDED"
	case intelQVLResultConfigAndSWHardeningNeeded:
		return "TEE_QV_RESULT_CONFIG_AND_SW_HARDENING_NEEDED"
	case intelQVLResultTDRelaunchAdvised:
		return "TEE_QV_RESULT_TD_RELAUNCH_ADVISED"
	case intelQVLResultTDRelaunchAdvisedConfigNeeded:
		return "TEE_QV_RESULT_TD_RELAUNCH_ADVISED_CONFIG_NEEDED"
	default:
		return fmt.Sprintf("TEE_QV_RESULT_UNKNOWN(0x%x)", verificationResult)
	}
}
