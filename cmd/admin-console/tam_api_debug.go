/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
)

type tamAPIDebugContextKey string

const tamAPIDebugFilenameContextKey tamAPIDebugContextKey = "tam-api-debug-filename"

func newTAMAPIClient() *http.Client {
	client := &http.Client{Timeout: 12 * time.Second}
	if conf.TAMAPIDebug {
		client.Transport = tamAPILoggingRoundTripper{
			next: http.DefaultTransport,
		}
	}
	return client
}

type tamAPILoggingRoundTripper struct {
	next http.RoundTripper
}

func (t tamAPILoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}

	reqBody, err := readAndRestoreBody(&req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body for debug logging: %w", err)
	}
	appLogger.Debug("TAM API request", "message", formatHTTPMessage(req.Method, req.URL.String(), req.Header, reqBody, redactedBodyPlaceholder(req)))

	resp, err := next.RoundTrip(req)
	if err != nil {
		appLogger.Debug("TAM API response error", "method", req.Method, "url", req.URL.String(), "err", err)
		return nil, err
	}

	respBody, err := readAndRestoreBody(&resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body for debug logging: %w", err)
	}
	appLogger.Debug("TAM API response", "status", resp.Status, "message", formatHTTPMessage("", "", resp.Header, respBody, ""))

	return resp, nil
}

func readAndRestoreBody(body *io.ReadCloser) ([]byte, error) {
	if body == nil || *body == nil {
		return nil, nil
	}
	raw, err := io.ReadAll(*body)
	if err != nil {
		return nil, err
	}
	_ = (*body).Close()
	*body = io.NopCloser(bytes.NewReader(raw))
	return raw, nil
}

func formatHTTPMessage(method string, url string, header http.Header, body []byte, redactedBody string) string {
	lines := make([]string, 0, 8)
	if method != "" || url != "" {
		lines = append(lines, fmt.Sprintf("%s %s", method, url))
	}
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		values := append([]string(nil), header.Values(k)...)
		sort.Strings(values)
		lines = append(lines, fmt.Sprintf("%s: %s", k, strings.Join(values, ", ")))
	}
	lines = append(lines, "body:")
	if redactedBody != "" {
		lines = append(lines, "  "+redactedBody)
	} else {
		lines = append(lines, indentBody(formatBodyForDebug(header.Get("Content-Type"), body)))
	}
	return strings.Join(lines, "\n")
}

func redactedBodyPlaceholder(req *http.Request) string {
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/SUITManifestService/RegisterManifest") {
		if filename, _ := req.Context().Value(tamAPIDebugFilenameContextKey).(string); filename != "" {
			return fmt.Sprintf("<omitted: %s>", filename)
		}
		return "<omitted>"
	}
	return ""
}

func formatBodyForDebug(contentType string, body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}
	if strings.HasPrefix(contentType, "application/cbor") || strings.Contains(contentType, "+cbor") {
		if diag, err := cbor.Diagnose(body); err == nil {
			return diag
		}
	}
	if isPrintableASCII(body) {
		return string(body)
	}
	return hex.EncodeToString(body)
}

func indentBody(body string) string {
	return "  " + strings.ReplaceAll(body, "\n", "\n  ")
}

func isPrintableASCII(body []byte) bool {
	for _, b := range body {
		if b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if b < 0x20 || b > 0x7e {
			return false
		}
	}
	return true
}
