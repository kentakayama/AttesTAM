/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package rats

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	quoteVersion3          = 3
	quoteVersion4          = 4
	quoteVersion5          = 5
	quoteHeaderVersionSize = 2
	quoteReportDataSize    = 64

	sgxReportBodySize          = 384
	sgxQuote3Size              = 436
	sgxQuote4Size              = 436
	sgxQuote5HeaderSize        = 48
	sgxQuote5TypeOffset        = 4
	sgxQuote5BodyLenOff        = 44
	sgxReportDataOffsetInBody  = 320
	sgxReportDataOffsetInQuote = 368
	intelQVLFMSPCSize          = 6
)

func verifyQuoteReportData(quote []byte, expected []byte) error {
	actual, err := extractQuoteReportData(quote)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return errors.New("quote report_data mismatch")
	}
	return nil
}

func ExtractQuoteReportData(quote []byte) ([]byte, error) {
	return extractQuoteReportData(quote)
}

func extractQuoteReportData(quote []byte) ([]byte, error) {
	if len(quote) < quoteHeaderVersionSize {
		return nil, errors.New("quote is too short")
	}

	version := binary.LittleEndian.Uint16(quote[:quoteHeaderVersionSize])
	switch version {
	case quoteVersion3:
		if len(quote) < sgxQuote3Size {
			return nil, errors.New("quote v3 is too short")
		}
		return copyQuoteReportData(quote[sgxReportDataOffsetInQuote : sgxReportDataOffsetInQuote+quoteReportDataSize]), nil
	case quoteVersion4:
		if len(quote) < sgxQuote4Size {
			return nil, errors.New("quote v4 is too short")
		}
		return copyQuoteReportData(quote[sgxReportDataOffsetInQuote : sgxReportDataOffsetInQuote+quoteReportDataSize]), nil
	case quoteVersion5:
		if len(quote) < sgxQuote5HeaderSize {
			return nil, errors.New("quote v5 is too short")
		}
		bodyType := binary.LittleEndian.Uint16(quote[sgxQuote5TypeOffset : sgxQuote5TypeOffset+2])
		bodySize := binary.LittleEndian.Uint32(quote[sgxQuote5BodyLenOff : sgxQuote5BodyLenOff+4])
		if len(quote) < sgxQuote5HeaderSize+int(bodySize) {
			return nil, errors.New("quote v5 body is truncated")
		}
		switch bodyType {
		case 1, 2, 3, 4:
			if bodySize < sgxReportBodySize {
				return nil, fmt.Errorf("quote v5 body type %d is too short for report_data", bodyType)
			}
			offset := sgxQuote5HeaderSize + sgxReportDataOffsetInBody
			return copyQuoteReportData(quote[offset : offset+quoteReportDataSize]), nil
		default:
			return nil, fmt.Errorf("unsupported quote v5 body type %d", bodyType)
		}
	default:
		return nil, fmt.Errorf("unsupported quote version %d", version)
	}
}

func copyQuoteReportData(in []byte) []byte {
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func extractIntelQVLFMSPC(quote []byte) ([]byte, error) {
	if len(quote) == 0 {
		return nil, fmt.Errorf("Intel QVL quote is empty")
	}
	fmspc, err := extractIntelQVLFMSPCNative(quote)
	if err != nil {
		return nil, err
	}
	if len(fmspc) != intelQVLFMSPCSize {
		return nil, fmt.Errorf("unexpected Intel QVL FMSPC size: got %d", len(fmspc))
	}
	return fmspc, nil
}

func intelQVLFMSPCQueryValue(fmspc []byte) (string, error) {
	if len(fmspc) != intelQVLFMSPCSize {
		return "", fmt.Errorf("unexpected Intel QVL FMSPC size: got %d", len(fmspc))
	}
	return strings.ToUpper(hex.EncodeToString(fmspc)), nil
}

func intelQVLFMSPCCacheKeyValue(fmspc []byte) (string, error) {
	if len(fmspc) != intelQVLFMSPCSize {
		return "", fmt.Errorf("unexpected Intel QVL FMSPC size: got %d", len(fmspc))
	}
	return strings.ToLower(hex.EncodeToString(fmspc)), nil
}
